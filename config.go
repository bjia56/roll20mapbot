package main

import "github.com/creasty/defaults"

type Config struct {
	Roll20Instances []struct {
		Roll20Email    string   `json:"roll20_email" default:"jdoe123@example.com"`
		Roll20Password string   `json:"roll20_password" default:"password"`
		Roll20Game     string   `json:"roll20_game" default:"My Game"`
		TargetChannels []string `json:"target_channels"`
	} `json:"roll20_instances"`
	DiscordToken   string `json:"discord_token" default:"ABC.123.XYZ"`
	DiscordStatus  string `json:"discord_status" default:""`
	Resolution     uint   `json:"resolution" default:"2000"`
	ViewportWidth  uint   `json:"viewport_width" default:"1280"`
	ViewportHeight uint   `json:"viewport_height" default:"720"`
	TimeDelay      uint   `json:"time_delay" default:"10"`
}

func DefaultConfig() Config {
	result := Config{}
	if err := defaults.Set(&result); err != nil {
		panic(err)
	}
	return result
}
