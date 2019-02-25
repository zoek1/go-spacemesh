package cmd

import (
	"fmt"
	bc "github.com/spacemeshos/go-spacemesh/config"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"os/signal"
)

type baseApp struct {
	Config *bc.Config
}

func newBaseApp() *baseApp {
	dc := bc.DefaultConfig()
	return &baseApp{Config: &dc}
}

func (app *baseApp) Initialize(cmd *cobra.Command) {
	// exit gracefully - e.g. with app Cleanup on sig abort (ctrl-c)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	// Goroutine that listens for Crtl ^ C command
	// and triggers the quit app
	go func() {
		for range signalChan {
			log.Info("Received an interrupt, stopping services...\n")
			Cancel()
		}
	}()

	// parse the config file based on flags et al
	conf, err := parseConfig()
	if err != nil {
		panic(err.Error())
	}

	app.Config = conf

	EnsureCLIFlags(cmd, app.Config)
}

func parseConfig() (*bc.Config, error) {
	fileLocation := viper.GetString("config")
	vip := viper.New()
	// read in default config if passed as param using viper
	if err := bc.LoadConfig(fileLocation, vip); err != nil {
		log.Error(fmt.Sprintf("couldn't load config file at location: %s swithing to defaults \n error: %v.",
			fileLocation, err))
		//return err
	}

	conf := bc.DefaultConfig()
	// load config if it was loaded to our viper
	err := vip.Unmarshal(&conf)
	if err != nil {
		log.Error("Failed to parse config\n")
		return nil, err
	}

	return &conf, nil
}