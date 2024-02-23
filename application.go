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

var msgCommandHandlers = map[string]func(*Application, *discordgo.Session, *discordgo.MessageCreate){
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
			logrus.Infof("Ignoring untracked channel %s", m.ChannelID)
			return
		}

		picture, err := r20.GetMap()
		if err != nil {
			logrus.Errorf("Error getting map: %s", err)
			return
		}

		_, err = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
			Files: []*discordgo.File{
				{Name: "map.jpg", Reader: picture},
			},
		})
		if err != nil {
			logrus.Errorf("Cannot post picture: %s", err)
			return
		}
	},
	"roll": func(a *Application, s *discordgo.Session, mc *discordgo.MessageCreate) {

	},
	"debuginfo": func(a *Application, s *discordgo.Session, m *discordgo.MessageCreate) {
		logrus.Info(spew.Sdump(m))
		s.ChannelMessageSend(m.ChannelID, "Debugging information printed to bot console.")
	},
}

func init() {
	// initialize shortcuts
	msgCommandHandlers["m"] = msgCommandHandlers["map"]
	msgCommandHandlers["r"] = msgCommandHandlers["roll"]
}

var slashCommands = []*discordgo.ApplicationCommand{
	{
		Name:        "map",
		Description: "Show roll20 map",
	},
	{
		Name:        "characters",
		Description: "List all character sheets on roll20",
	},
	{
		Name:        "sheet",
		Description: "Show character sheet",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "name",
				Description: "Name of the character sheet",
				Required:    true,
			},
		},
	},
}

var slashCommandHandlers = map[string]func(*Application, *discordgo.Session, *discordgo.InteractionCreate){
	"map": func(app *Application, s *discordgo.Session, i *discordgo.InteractionCreate) {
		r20, ok := app.Roll20ChannelMap[i.ChannelID]
		if !ok {
			logrus.Infof("Ignoring untracked channel %s", i.ChannelID)
			err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Channel is untracked",
				},
			})
			if err != nil {
				logrus.Errorf("Error responding: %s", err)
			}
			return
		}

		picture, err := r20.GetMap()
		if err != nil {
			logrus.Errorf("Error getting map: %s", err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Error getting map",
				},
			})
			if err != nil {
				logrus.Errorf("Error responding: %s", err)
			}
			return
		}

		err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Files: []*discordgo.File{
					{Name: "map.jpg", Reader: picture},
				},
			},
		})
		if err != nil {
			logrus.Errorf("Cannot post picture: %s", err)
			return
		}
	},
	"characters": func(app *Application, s *discordgo.Session, ic *discordgo.InteractionCreate) {
		r20, ok := app.Roll20ChannelMap[ic.ChannelID]
		if !ok {
			logrus.Infof("Ignoring untracked channel %s", ic.ChannelID)
			err := s.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Channel is untracked",
				},
			})
			if err != nil {
				logrus.Errorf("Error responding: %s", err)
			}
			return
		}

		csList, err := r20.ListCharacterSheets()
		if err != nil {
			logrus.Errorf("Error getting character sheets: %s", err)
			s.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Error getting character sheets",
				},
			})
			if err != nil {
				logrus.Errorf("Error responding: %s", err)
			}
			return
		}

		err = s.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprintf("```\n%s\n```", strings.Join(csList, "\n")),
			},
		})
		if err != nil {
			logrus.Errorf("Error responding: %s", err)
			return
		}
	},
	"sheet": func(app *Application, s *discordgo.Session, ic *discordgo.InteractionCreate) {
		r20, ok := app.Roll20ChannelMap[ic.ChannelID]
		if !ok {
			logrus.Infof("Ignoring untracked channel %s", ic.ChannelID)
			err := s.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Channel is untracked",
				},
			})
			if err != nil {
				logrus.Errorf("Error responding: %s", err)
			}
			return
		}

		character := ic.ApplicationCommandData().Options[0].StringValue()
		cs, err := r20.GetCharacterSheet(character)
		if err != nil {
			logrus.Errorf("Error getting character sheet: %s", err)
			err = s.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Error getting character sheet",
				},
			})
			if err != nil {
				logrus.Errorf("Error responding: %s", err)
			}
			return
		}

		err = s.InteractionRespond(ic.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Files: []*discordgo.File{
					{Name: fmt.Sprintf("%s.pdf", character), Reader: cs},
				},
			},
		})
		if err != nil {
			logrus.Errorf("Error responding: %s", err)
			return
		}
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
		r20 := NewRoll20Browser(cfg.Roll20Email, cfg.Roll20Password, cfg.Roll20Game, config.Resolution, config.ViewportWidth, config.ViewportHeight)
		for _, target := range cfg.TargetChannels {
			if _, ok := app.Roll20ChannelMap[target]; ok {
				panic(fmt.Errorf("channel %s is tracking multiple roll20 instances", target))
			}
			app.Roll20ChannelMap[target] = r20
		}
		app.Roll20Instances = append(app.Roll20Instances, r20)
	}
	app.Discord = NewDiscordBot(config.DiscordToken, config.DiscordStatus, app.DiscordMessageCreateHandler(), slashCommands, app.DiscordInteractionCreateHandler())
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
				time.Sleep(time.Second * 10)
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

func (app *Application) DiscordMessageCreateHandler() MsgHandler {
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

		f, ok := msgCommandHandlers[command]

		// ignore unknown commands
		if !ok {
			return
		}

		// run command
		go f(app, s, m)
	}
}

func (app *Application) DiscordInteractionCreateHandler() SlashHandler {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := slashCommandHandlers[i.ApplicationCommandData().Name]; ok {
			go h(app, s, i)
		} else {
			logrus.Errorf("Unknown slash command %s", i.ApplicationCommandData().Name)
		}
	}
}
