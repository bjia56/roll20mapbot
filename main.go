package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func main() {
	log.SetPrefix("")
	log.SetFlags(0)
	log.SetOutput(logrus.StandardLogger().Writer())

	executeCobra()
}

func executeCobra() {
	displaySpec := false
	configFile := ""

	var rootCmd = &cobra.Command{
		Use:   "roll20mapbot",
		Short: "A Discord bot to read D&D maps from roll20 games",
		Run: func(cmd *cobra.Command, args []string) {
			if displaySpec {
				encoder := json.NewEncoder(os.Stdout)
				err := encoder.Encode(DefaultConfig())
				if err != nil {
					logrus.Fatalf("could not generate default config: %s", err)
				}
				return
			}

			if configFile == "" {
				logrus.Fatalf("config file required")
			}

			config, err := loadConfig(configFile)
			if err != nil {
				logrus.Fatalf("could not load config: %s", err)
			}

			app := NewApplication(config)
			err = app.Launch()
			if err != nil {
				logrus.Fatalf("could not launch application: %s", err)
			}
			defer app.Close()

			// wait for exit
			sc := make(chan os.Signal, 1)
			signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
			<-sc

			logrus.Printf("Shutting down and exiting")
		},
	}

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file")
	rootCmd.PersistentFlags().BoolVar(&displaySpec, "spec", false, "display config specification and exit")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadConfig(configFile string) (Config, error) {
	cf, err := os.Open(configFile)
	if err != nil {
		return Config{}, err
	}

	result := DefaultConfig()

	decoder := json.NewDecoder(cf)
	err = decoder.Decode(&result)
	if err != nil {
		return Config{}, err
	}

	return result, nil
}
