package main

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type DiscordBot struct {
	token  string
	status string

	session *discordgo.Session
	handler func(*discordgo.Session, *discordgo.MessageCreate)
}

func NewDiscordBot(token, status string, handler func(*discordgo.Session, *discordgo.MessageCreate)) *DiscordBot {
	return &DiscordBot{
		token:   token,
		status:  status,
		handler: handler,
	}
}

func (d *DiscordBot) Launch() error {
	var err error

	// set up Discord bot
	logrus.Printf("Setting up Discord bot")
	d.session, err = discordgo.New("Bot " + d.token)
	if err != nil {
		return fmt.Errorf("error creating Discord session: %w", err)
	}

	d.session.AddHandler(d.handler)
	d.session.Identify.Intents = discordgo.IntentsGuildMessages
	err = d.session.Open()
	if err != nil {
		return fmt.Errorf("error opening websocket connection: %w", err)
	}

	err = d.session.UpdateGameStatus(0, d.status)
	if err != nil {
		return fmt.Errorf("error setting game status: %w", err)
	}

	logrus.Printf("Discord bot is ready")

	return nil
}

func (d *DiscordBot) Close() {
	if err := d.session.Close(); err != nil {
		panic(err)
	}
}
