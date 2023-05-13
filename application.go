package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/davecgh/go-spew/spew"
	"github.com/sirupsen/logrus"
)

var prefix = '%'

var commands = map[string]func(*Application, *discordgo.Session, *discordgo.MessageCreate){
	"ping": func(app *Application, s *discordgo.Session, m *discordgo.MessageCreate) {
		start := time.Now()

		sent, err := s.ChannelMessageSend(m.ChannelID, "Pong!")
		if err != nil {
			logrus.Errorf("Error responding to ping: %s", err)
			return
		}

		elapsed := time.Since(start)

		_, err = s.ChannelMessageEdit(sent.ChannelID, sent.ID, fmt.Sprintf("Pong! **%s**", elapsed.String()))
		if err != nil {
			logrus.Errorf("Error editing ping message: %s", err)
			return
		}
	},
	"map": func(app *Application, s *discordgo.Session, m *discordgo.MessageCreate) {
		r20, ok := app.Roll20ChannelMap[m.ChannelID]
		if !ok {
			logrus.Info("Ignoring untracked channel %s", m.ChannelID)
			return
		}

		picture, err := r20.GetMap(true)
		if err != nil {
			logrus.Errorf("Error getting map with spam protection: %s", err)
			return
		}

		_, err = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Files: []*discordgo.File{
				{Name: "map.png", Reader: picture},
			},
		})
		if err != nil {
			logrus.Errorf("Cannot post picture: %s", err)
			return
		}
	},
	"reload": func(app *Application, s *discordgo.Session, m *discordgo.MessageCreate) {
		_, err := s.ChannelMessageSend(m.ChannelID, "Reloading roll20 browser(s), this may take a minute...")
		if err != nil {
			logrus.Errorf("Error responding to reload: %s", err)
			return
		}

		for _, r20 := range app.Roll20Instances {
			err = r20.Relaunch()
			if err != nil {
				logrus.Errorf("Error reloading roll20: %s", err)
				_, err := s.ChannelMessageSend(m.ChannelID, "Error reloading roll20. A restart may be required.")
				if err != nil {
					logrus.Errorf("Error sending error message: %s", err)
					return
				}
				return
			}
		}

		_, err = s.ChannelMessageSend(m.ChannelID, "Successfully reloaded roll20 browser(s).")
		if err != nil {
			logrus.Errorf("Error sending success message: %s", err)
			return
		}
	},
	"debuginfo": func(a *Application, s *discordgo.Session, m *discordgo.MessageCreate) {
		logrus.Info(spew.Sdump(m))
		s.ChannelMessageSend(m.ChannelID, "Debugging information printed to bot console.")
	},
}

type Application struct {
	Config
	Roll20ChannelMap map[string]*Roll20Browser
	Roll20Instances  []*Roll20Browser
	Discord          *DiscordBot

	closed bool
}

func NewApplication(config Config) *Application {
	app := &Application{
		Config:           config,
		Roll20ChannelMap: make(map[string]*Roll20Browser),
	}
	for _, cfg := range config.Roll20Instances {
		r20 := NewRoll20Browser(cfg.Roll20Email, cfg.Roll20Password, cfg.Roll20Game, config.HDResolution, config.StandardResolution)
		for _, target := range cfg.TargetChannels {
			if _, ok := app.Roll20ChannelMap[target]; ok {
				panic(fmt.Errorf("channel %s is tracking multiple roll20 instances", target))
			}
			app.Roll20ChannelMap[target] = r20
		}
		app.Roll20Instances = append(app.Roll20Instances, r20)
	}
	app.Discord = NewDiscordBot(config.DiscordToken, config.DiscordStatus, app.DiscordMessageCreateHandler())
	return app
}

func (app *Application) Launch() error {
	var err error

	for _, r20 := range app.Roll20Instances {
		err = r20.Launch()
		if err != nil {
			return fmt.Errorf("error launching roll20: %w", err)
		}
	}

	err = app.Discord.Launch()
	if err != nil {
		return fmt.Errorf("error launching Discord: %w", err)
	}

	logrus.Printf("Application is ready")
	go app.periodicRelaunch()

	return nil
}

func (app *Application) periodicRelaunch() {
	time.Sleep(time.Minute * 40)
	for !app.closed {
		logrus.Printf("Starting periodic reload")
		for _, r20 := range app.Roll20Instances {
			err := r20.Relaunch()
			if err != nil {
				logrus.Errorf("Error reloading roll20: %s", err)
				continue
			}
		}
		time.Sleep(time.Minute * 40)
	}
}

func (app *Application) Close() {
	app.closed = true
	for _, r20 := range app.Roll20Instances {
		r20.Close()
	}
	app.Discord.Close()
}

func (app *Application) DiscordMessageCreateHandler() func(*discordgo.Session, *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		// ignore all messages created by the bot itself
		if m.Author.ID == s.State.User.ID {
			return
		}

		// ignore empty messages
		if m.Content == "" {
			return
		}

		runes := []rune(m.Content)

		// ignore if first rune is not the prefix
		if runes[0] != prefix {
			return
		}

		command := strings.ToLower(string(runes[1:]))

		f, ok := commands[command]

		// ignore unknown commands
		if !ok {
			return
		}

		// run command
		go f(app, s, m)
	}
}
