package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

const (
	APPLICATION_ID = 1169055366085087312
	PUBLIC_KEY     = 0x54b745e43fd79dd0e3d8f494c1d12625adaa516af6551da56ca62c220bcde863
	PERMS_INTEGER  = 8
	GUILD_ID       = "1157158258621042799"
)

var ready = false

func main() {
	token := os.Getenv("DISCORD_API_TOKEN")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal("unable to initialize a new discordgo client")
	}
	defer dg.Close()

	if err != nil {
		log.Fatal("unable to get voice state")
	}

	channels, err := dg.GuildChannels(GUILD_ID)
	if err != nil {
		log.Fatal("unable to get channels")
	}
	wagecages := []*discordgo.Channel{}
	for _, c := range channels {
		if c.Type == discordgo.ChannelTypeGuildVoice {
			if strings.HasPrefix(c.Name, "WAGE CAGE") {
				wagecages = append(wagecages, c)
			}
		}
	}

	if len(wagecages) == 0 {
		fmt.Println("line 41")
		newchan, err := createWagecage(dg, 0)
		if err != nil {
			log.Fatal("unable to create first wage cage")
		}
		wagecages = append(wagecages, newchan)
	}
	dg.Identify.Intents = discordgo.IntentsGuildVoiceStates | discordgo.IntentsGuildMembers
	dg.AddHandler(func (s *discordgo.Session, r *discordgo.Ready){
		fmt.Println(r.Version)
		fmt.Println(s.DataReady)
		ready = true
	})

	err = dg.Open()
	if err != nil {
		log.Fatalf("error opening ws connection: %s", err.Error())
	}
	for !ready{}
	fmt.Println("pre-data-ready")
	for !dg.DataReady {
	}
	fmt.Println("post-data-ready")
	defer dg.Close()
	fmt.Printf("tracking voice: %b", dg.State.TrackVoice)
	Vstate, err := dg.State.VoiceState(GUILD_ID, "444210585605767183")
	if err != nil{
		log.Fatalf("something bad happened: %s", err.Error())
	}
	fmt.Printf("lvcky voice state: %s", Vstate.ChannelID)
	// if there's no one in them, delete the empties (and roles) save for the first one
	// else for each populated wage cage
	// give them a new wage cage admin role for their channel
	// if all are filled up
	// create another wage cage
}

func createWagecage(dg *discordgo.Session, id int) (*discordgo.Channel, error) {
	ch, err := dg.GuildChannelCreate(GUILD_ID, fmt.Sprintf("WAGE CAGE #%d", id), discordgo.ChannelTypeGuildVoice)
	if err != nil {
		return nil, err
	}

	return ch, nil
}
