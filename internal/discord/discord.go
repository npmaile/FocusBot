package discord

import (
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/npmaile/focusbot/internal/db"
	"github.com/npmaile/focusbot/internal/guild"
	"github.com/npmaile/focusbot/internal/models"
	"github.com/npmaile/focusbot/pkg/logerooni"
)

func InitializeDG(servers []*guild.Guild, token string, db db.DataStore) (*models.GlobalConfig, error) {
	logerooni.Debug("InitializeDG called")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		logerooni.Errorf("unable to initialize a new discordgo client: " + err.Error())
	}

	dg.SyncEvents = false
	dg.LogLevel = discordgo.LogInformational
	dg.ShouldReconnectOnError = true
	dg.ShouldRetryOnRateLimit = true
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
	mg := mtexGuilds{
		g:    map[string]*guild.Guild{},
		mtex: sync.Mutex{},
	}
	for _, s := range servers {
		mg.g[s.Config.ID] = s
	}

	dg.AddHandler(GuildCreateHandlerFunc(&mg, db))
	dg.AddHandler(GuildMembersChunkFunc(&mg))
	dg.AddHandler(GuildVoiceStateUpdateHandlerFunc(&mg))
	discordgo.Logger = logerooni.DiscordLogger

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
func GuildCreateHandlerFunc(mg *mtexGuilds, db db.DataStore) func(_ *discordgo.Session, gc *discordgo.GuildCreate) {
	return func(dg *discordgo.Session, gc *discordgo.GuildCreate) {
		logerooni.Debugf("GuildCreateHandler Event fired for guild %s", gc.ID)
		server, ok := mg.lookupguild("GuildCreateHandlerFunc", gc.ID)
		if !ok {
			var err error
			server, err = AdHocSetUpGuild(mg, db, gc.ID, dg)
			if err != nil {
				logerooni.Errorf("unable to route VoiceStateUpdate to guild process %s, guildID not in process store", gc.ID)
				return
			}
		}
		server.GuildChan <- gc
		logerooni.Debugf("Routed GuildCreate to guild process %s", gc.ID)
	}
}

func AdHocSetUpGuild(mg *mtexGuilds, db db.DataStore, ID string, dg *discordgo.Session) (*guild.Guild, error) {
	logerooni.Infof("AdHocSetUpGuild called for guildID: %s", ID)
	guildConfig, err := db.GetServerConfiguration(ID)
	if err == nil {
		return guild.NewFromConfig(&guildConfig), nil
	}
	guildConfig = models.DefaultGuildConfig(ID)
	err = db.AddServer(guildConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to store new guild %s", ID)
	}
	server := guild.NewFromConfig(&guildConfig)
	mg.addGuild(server)
	go server.SetOffServerProcessing(dg)
	return server, nil
}

// 1. add the new guild to the database
// 2. start processing the guild

// called when asked for and necessary for getting membership information
func GuildMembersChunkFunc(mg *mtexGuilds) func(_ *discordgo.Session, gm *discordgo.GuildMembersChunk) {
	return func(_ *discordgo.Session, gm *discordgo.GuildMembersChunk) {
		logerooni.Debugf("GuildMembersChunck Event fired for guild %s", gm.GuildID)
		server, ok := mg.lookupguild("GuildMembersChunkFunc", gm.GuildID)
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
		g, ok := mg.lookupguild("GuildVoiceStateUpdateHandlerFunc", vs.GuildID)
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

func (m *mtexGuilds) lookupguild(caller string, s string) (*guild.Guild, bool) {
	logerooni.Debugf("[lookupguild(%s)] %s requested lookup.. locking", caller, caller)
	m.mtex.Lock()
	g, ok := m.g[s]
	logerooni.Debugf("[lookupguild(%s)] Got guildid value %v, ok: %v", caller, g, ok)
	m.mtex.Unlock()
	return g, ok
}

func (m *mtexGuilds) addGuild(guild *guild.Guild) {
	m.mtex.Lock()
	m.g[guild.Config.ID] = guild
	m.mtex.Unlock()
}
