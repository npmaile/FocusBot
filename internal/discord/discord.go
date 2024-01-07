package discord

import (
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/npmaile/wagebot/internal/guild"
	"github.com/npmaile/wagebot/internal/models"
	"github.com/npmaile/wagebot/pkg/logerooni"
)

func InitializeDG(servers []*guild.Guild, token string) (*models.GlobalConfig, error) {
	logerooni.Debug("InitializeDG called")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		logerooni.Errorf("unable to initialize a new discordgo client: " + err.Error())
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

	dg.ShouldReconnectOnError = true

	readychan := make(chan *discordgo.Ready)
	dg.AddHandler(ReadyHandlerFunc(readychan))
	mg := mtexGuilds{
		g:    map[string]*guild.Guild{},
		mtex: sync.Mutex{},
	}
	for _, s := range servers {
		mg.g[s.Config.ID] = s
	}

	dg.AddHandler(GuildCreateHandlerFunc(&mg))
	dg.AddHandler(GuildMembersChunkFunc(&mg))
	dg.AddHandler(GuildVoiceStateUpdateHandlerFunc(&mg))
	dg.LogLevel = discordgo.LogDebug


	logerooni.Debug("Opening discordgo websocket connection")
	err = dg.Open()
	if err != nil {
		return nil, fmt.Errorf("unable to open websocket to discord: %s", err.Error())
	}

	var ready *discordgo.Ready
	logerooni.Debug("waiting for ready signal from discord websocket connection")
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
		logerooni.Debug("Received Ready! event from discord websocket api")
		readychan <- r
	}
}

// called at the beginning for each guild connected
func GuildCreateHandlerFunc(mg *mtexGuilds) func(_ *discordgo.Session, gc *discordgo.GuildCreate) {
	return func(_ *discordgo.Session, gc *discordgo.GuildCreate) {
		logerooni.Debugf("GuildCreateHandler Event fired for guild %s", gc.ID)
		server, ok := mg.lookupguild(gc.ID)
		if !ok {
			logerooni.Errorf("unable to route VoiceStateUpdate to guild process %s, guildID not in process store", gc.ID)
			return
		}
		server.GuildChan <- gc
		logerooni.Debugf("Routed GuildCreate to guild process %s", gc.ID)
	}
}

// called when asked for and necessary for getting membership information
func GuildMembersChunkFunc(mg *mtexGuilds) func(_ *discordgo.Session, gm *discordgo.GuildMembersChunk) {
	return func(_ *discordgo.Session, gm *discordgo.GuildMembersChunk) {
		logerooni.Debugf("GuildMembersChunck Event fired for guild %s", gm.GuildID)
		server, ok := mg.lookupguild(gm.GuildID)
		if !ok {
			logerooni.Errorf("unable to route GuildMembersChunk to guild process %s, guildID not in process store", gm.GuildID)
			return
		}
		server.MembersChan <- gm
		logerooni.Debugf("Routed GuildMembersChunk to guild process %s", gm.GuildID)
	}
}

func GuildVoiceStateUpdateHandlerFunc(mg *mtexGuilds) func(_ *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
	return func(_ *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
		logerooni.Debugf("GuildVoiceStateUpate Event fired for guild %s", vs.GuildID)
		g, ok := mg.lookupguild(vs.GuildID)
		if !ok {
			logerooni.Errorf("unable to route VoiceStateUpdate to guild process %s, guildID not in process store", vs.GuildID)
			return
		}
		g.VoiceStateUpdate <- vs
		logerooni.Debugf("Routed VoiceStateUpdate to guild process %s", vs.GuildID)
	}
}

//todo: add a handler to handle channel creation that routes to the proper guildprocessing process to get rid of the time.wait

type mtexGuilds struct {
	g    map[string]*guild.Guild
	mtex sync.Mutex
}

func (m *mtexGuilds) lookupguild(s string) (*guild.Guild, bool) {
	m.mtex.Lock()
	g, ok := m.g[s]
	m.mtex.Unlock()
	return g, ok
}
