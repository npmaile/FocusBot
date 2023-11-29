package discord

import (
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/npmaile/wagebot/internal/guild"
	"github.com/npmaile/wagebot/internal/models"
)

func InitializeDG(servers []guild.Guild, token string) (*models.GlobalConfig, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal("unable to initialize a new discordgo client: " + err.Error())
	}
	dg.SyncEvents = false
	dg.StateEnabled = true
	dg.State.TrackChannels = true
	dg.State.TrackMembers = true
	dg.State.TrackRoles = true
	dg.State.TrackVoice = true
	dg.State.TrackPresences = true

	// todo: get only the necessary intents
	dg.Identify.Intents = discordgo.IntentsAll

	readychan := make(chan *discordgo.Ready)
	dg.AddHandler(ReadyHandlerFunc(readychan))
	dg.AddHandler(GuildCreateHandlerFunc(servers))
	dg.AddHandler(GuildMembersChunkFunc(servers))

	// global
	err = dg.Open()
	if err != nil {
		return nil, fmt.Errorf("unable to open websocket to discord: %s", err.Error())
	}

	var ready *discordgo.Ready
	select {
	case ready = <-readychan:
		break
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("unable to receive discord ready signal after 10 seconds")
	}

	return &models.GlobalConfig{
		Ready: ready,
		DG:    dg,
	}, nil
}

func ReadyHandlerFunc(readychan chan *discordgo.Ready) func(_ *discordgo.Session, r *discordgo.Ready) {
	return func(_ *discordgo.Session, r *discordgo.Ready) {
		fmt.Println("Ready!")
		readychan <- r
	}
}

func GuildMembersChunkFunc(servers []guild.Guild) func(_ *discordgo.Session, gm *discordgo.GuildMembersChunk) {
	return func(_ *discordgo.Session, gm *discordgo.GuildMembersChunk) {
		fmt.Println("received guild members")
		for _, server := range servers {
			if server.ID == gm.GuildID {
				server.MembersChan <- gm
				fmt.Println("sent guild members chunk")
				return
			}
		}
	}
}

func GuildCreateHandlerFunc(servers []guild.Guild) func(_ *discordgo.Session, gc *discordgo.GuildCreate) {
	return func(_ *discordgo.Session, gc *discordgo.GuildCreate) {
		fmt.Println("received guild create")
		for _, server := range servers {
			if server.ID == gc.ID {
				server.GuildChan <- gc
				fmt.Println("sent guild create")
				return
			}
		}
	}
}
