package main

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

type MsgHandler = func(*discordgo.Session, *discordgo.MessageCreate)
type SlashHandler = func(*discordgo.Session, *discordgo.InteractionCreate)

type DiscordBot struct {
	token  string
	status string

	session    *discordgo.Session
	msgHandler MsgHandler

	slashCmds    []*discordgo.ApplicationCommand
	slashHandler SlashHandler
}

func NewDiscordBot(token, status string, msgHandler MsgHandler, slashCmds []*discordgo.ApplicationCommand, slashHandler SlashHandler) *DiscordBot {
	return &DiscordBot{
		token:        token,
		status:       status,
		msgHandler:   msgHandler,
		slashCmds:    slashCmds,
		slashHandler: slashHandler,
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

	d.session.AddHandler(d.msgHandler)
	d.session.AddHandler(d.slashHandler)
	d.session.Identify.Intents = discordgo.IntentsGuildMessages

	err = d.session.Open()
	if err != nil {
		return fmt.Errorf("error opening websocket connection: %w", err)
	}

	err = d.session.UpdateGameStatus(0, d.status)
	if err != nil {
		return fmt.Errorf("error setting game status: %w", err)
	}

	for _, c := range d.slashCmds {
		_, err := d.session.ApplicationCommandCreate(d.session.State.User.ID, "", c)
		if err != nil {
			return fmt.Errorf("could not create slash command %s: %w", c.Name, err)
		}
	}

	logrus.Printf("Discord bot is ready")

	return nil
}

func (d *DiscordBot) Close() {
	if err := d.session.Close(); err != nil {
		panic(err)
	}
}
