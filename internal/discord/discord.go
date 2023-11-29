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

	readychan := make(chan *discordgo.Ready)
	dg.AddHandler(ReadyHandlerFunc(readychan))
	dg.AddHandler(GuildCreateHandlerFunc(servers))
	dg.AddHandler(GuildMembersChunkFunc(servers))
	mg := mtexGuilds{
		g:    map[string]*guild.Guild{},
		mtex: sync.Mutex{},
	}
	for _, s := range servers {
		mg.g[s.ID] = s
	}
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
func GuildMembersChunkFunc(servers []*guild.Guild) func(_ *discordgo.Session, gm *discordgo.GuildMembersChunk) {
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

// called at the beginning for each guild connected
func GuildCreateHandlerFunc(servers []*guild.Guild) func(_ *discordgo.Session, gc *discordgo.GuildCreate) {
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

func GuildVoiceStateUpdateHandlerFunc(mg *mtexGuilds) func(_ *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
	return func(_ *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
		fmt.Println("about to route a voice state update")
		mg.mtex.Lock()
		defer mg.mtex.Unlock()
		g, ok := mg.g[vs.GuildID]
		if !ok {
			fmt.Println("failed to get a guild from the list")
			// ignore this
			return
		}
		g.VoiceStateUpdate <- vs
		fmt.Println("just routed a voice state update")

	}
}

type mtexGuilds struct {
	g    map[string]*guild.Guild
	mtex sync.Mutex
}
