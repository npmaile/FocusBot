package discord

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/npmaile/wagebot/internal/guild"
	"github.com/npmaile/wagebot/internal/models"
)

func InitializeDG(servers []*guild.Guild, token string) (*models.GlobalConfig, error) {
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

	mg := mtexGuilds{
		g:    map[string]*guild.Guild{},
		mtex: sync.Mutex{},
	}
	readychan := make(chan *discordgo.Ready)
	dg.AddHandler(ReadyHandlerFunc(readychan))
	dg.AddHandler(GuildCreateHandlerFunc(&mg))
	dg.AddHandler(GuildMembersChunkFunc(&mg))

	dg.AddHandler(GuildVoiceStateUpdateHandlerFunc(&mg))

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

// called once at the beginning
func ReadyHandlerFunc(readychan chan *discordgo.Ready) func(_ *discordgo.Session, r *discordgo.Ready) {
	return func(_ *discordgo.Session, r *discordgo.Ready) {
		fmt.Println("Ready!")
		readychan <- r
	}
}

// called when asked for and necessary for getting membership information
func GuildMembersChunkFunc(mg *mtexGuilds) func(_ *discordgo.Session, gm *discordgo.GuildMembersChunk) {
	return func(_ *discordgo.Session, gm *discordgo.GuildMembersChunk) {
		fmt.Println("received guild members")
		mg.mtex.Lock()
		server, ok := mg.g[gm.GuildID]
		mg.mtex.Unlock()
		if !ok {
			fmt.Println("failed to get a guild from the list")
			// ignore this
			return
		}
		server.MembersChan <- gm
		fmt.Println("sent guild members chunk")
	}
}

// called at the beginning for each guild connected
func GuildCreateHandlerFunc(mg *mtexGuilds) func(_ *discordgo.Session, gc *discordgo.GuildCreate) {
	return func(_ *discordgo.Session, gc *discordgo.GuildCreate) {
		fmt.Println("received guild create")
		mg.mtex.Lock()
		server, ok := mg.g[gc.ID]
		mg.mtex.Unlock()
		if !ok {
			fmt.Println("failed to get a guild from the list")
			// ignore this
			// TODO: this (it's important for onboarding)
			return
		}
		server.GuildChan <- gc
		fmt.Println("sent guild create")
	}
}

func GuildVoiceStateUpdateHandlerFunc(mg *mtexGuilds) func(_ *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
	return func(_ *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
		fmt.Println("about to route a voice state update")
		mg.mtex.Lock()
		g, ok := mg.g[vs.GuildID]
		mg.mtex.Unlock()
		if !ok {
			fmt.Println("failed to get a guild from the list")
			// ignore this
			return
		} //todo: fix this

		g.VoiceStateUpdate <- vs
		fmt.Println("just routed a voice state update")

	}
}

//todo: add a handler to handle channel creation that routes to the proper guildprocessing process to get rid of the time.wait

type mtexGuilds struct {
	g    map[string]*guild.Guild
	mtex sync.Mutex
}
