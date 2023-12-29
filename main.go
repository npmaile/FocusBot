package main

import (
	"log"
	"net/http"
	"os"

	"github.com/npmaile/wagebot/internal/db"
	"github.com/npmaile/wagebot/internal/discord"
	webServer "github.com/npmaile/wagebot/internal/web_server"

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
		//todo: better logging library and configuraion to handle log levels
		log.Fatalf("unable to read configuration: %s", err.Error())
	}

	log.Println("starting up")
	clientID := viper.GetString("bot.app_id")
	token := viper.GetString("bot.api_token")

	//todo: add other database backend options to configuration
	db, err := db.NewSqliteStore(viper.GetString("store.path"))
	if err != nil {
		log.Fatalf("unable to start: %s", err.Error())
	}
	err = discord.InitializeDG(db, token)
	if err != nil {
		log.Fatalf("unable to initialize discordgo client: %s", err.Error())
	}

	// todo: the entire management interface
	log.Println("Listening on :8080")
	http.HandleFunc("/link", webServer.ServeLinkPageFunc(clientID))
	http.ListenAndServe(":8080", nil)
}
