package main

import (
	"net/http"
	"os"

	"github.com/npmaile/focusbot/internal/db"
	"github.com/npmaile/focusbot/internal/discord"
	"github.com/npmaile/focusbot/internal/guild"
	webServer "github.com/npmaile/focusbot/internal/web_server"
	"github.com/npmaile/focusbot/pkg/logerooni"

	"github.com/spf13/viper"
)

const version = "1"

func main() {
	logerooni.Info("starting wagebot version " + version)
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "."
	}
	// config boilerplate
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(configPath)
	err := viper.ReadInConfig()
	if err != nil {
		logerooni.Fatalf("unable to read configuration: %s", err.Error())
	}

	logerooni.Info("starting up")
	clientID := viper.GetString("bot.app_id")
	token := viper.GetString("bot.api_token")
	certfile := viper.GetString("http.certfile")
	keyfile := viper.GetString("http.keyfile")

	//todo: add other database backend options to configuration
	db, err := db.NewSqliteStore(viper.GetString("store.path"))
	if err != nil {
		logerooni.Fatalf("unable to start: %s", err.Error())
	}

	serverConfigs, err := db.GetAllServerConfigs()
	if err != nil {
		logerooni.Fatalf("unable to start: %s", err.Error())
	}
	//todo: abstract this out to another package
	servers := []*guild.Guild{}
	for _, config := range serverConfigs {
		g := guild.NewFromConfig(config)
		servers = append(servers, g)
	}

	dg, err := discord.InitializeDG(servers, token, db)
	if err != nil {
		logerooni.Fatalf("unable to initialize discordgo client: %s", err.Error())
	}

	for _, s := range servers {
		//todo: set off server processing for any servers whose handlers crash
		go s.SetOffServerProcessing(dg.DG)
	}

	webServer.SetupWebServer(clientID)
	if certfile != "" && keyfile != "" {
		logerooni.Info("Listening on :443")
		err = http.ListenAndServeTLS(":443", certfile, keyfile, nil)
	} else {
		logerooni.Info("Listening on :80")
		err = http.ListenAndServe(":80", nil)
	}
	if err != nil {
		logerooni.Fatalf("Web server unexpectedly quit: %s", err.Error())
	}
}
