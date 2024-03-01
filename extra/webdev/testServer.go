package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/npmaile/focusbot/internal/db"
	webServer "github.com/npmaile/focusbot/internal/web_server"
	"github.com/npmaile/focusbot/pkg/logerooni"
	"github.com/spf13/viper"
)

func main() {
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
	oAuth2ClientSecret := viper.GetString("bot.Oauth2ClientSecret")
	db, err := db.NewSqliteStore(viper.GetString("store.path"))
	if err != nil {
		logerooni.Fatalf("unable to start: %s", err.Error())
	}

	webServer.SetupWebServer(clientID, oAuth2ClientSecret, db)

	logerooni.Info("Listening on :80")
	fmt.Println(http.ListenAndServe(":80", nil).Error())
}
