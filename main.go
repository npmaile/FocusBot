package main

import (
	"log"
	"net/http"
	"os"

	"github.com/npmaile/wagebot/internal/db"
	"github.com/npmaile/wagebot/internal/discord"
	"github.com/npmaile/wagebot/internal/guild"
	webServer "github.com/npmaile/wagebot/internal/web_server"

	"github.com/spf13/viper"
)

func main() {
	// config boilerplate
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		//todo: better logging library and configuraion to handle log levels
		log.Fatalf("unable to read configuration: %s", err.Error())
	}

	log.Println("starting up")
	clientID := viper.GetString("bot.app_id")

	token := os.Getenv("DISCORD_API_TOKEN")

	//todo: add other database backend options to configuration
	db, err := db.NewSqliteStore("./db")
	if err != nil {
		log.Fatalf("unable to start: %s", err.Error())
	}

	serverConfigs, err := db.GetAllServerConfigs()
	if err != nil {
		log.Fatalf("unable to start: %s", err.Error())
	}
	//todo: abstract this out to another package
	servers := []*guild.Guild{}
	for _, config := range serverConfigs {
		g := guild.NewFromConfig(config)
		servers = append(servers, g)
	}

	dg, err := discord.InitializeDG(servers, token)
	if err != nil {
		log.Fatalf("unable to initialize discordgo client: %s", err.Error())
	}

	for _, s := range servers {
		//todo: set this off for new servers coming into the system
		//todo: set off server processing for any servers whose handlers crash
		go s.SetOffServerProcessing(dg.DG)
	}

	// todo: the entire management interface
	log.Println("Listening on :8080")
	http.HandleFunc("/link", webServer.ServeLinkPageFunc(clientID))
	http.ListenAndServe(":8080", nil)
}
