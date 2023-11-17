package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/npmaile/wagebot/internal/db"
	"github.com/npmaile/wagebot/internal/models"
	"github.com/npmaile/wagebot/internal/server"

	"github.com/bwmarrin/discordgo"
	"github.com/spf13/viper"
)

const (
	PERMS_INTEGER = 8
	GUILD_ID      = "1157158258621042799"
)

func main() {
	// config boilerplate
	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("unable to read configuration: %s", err.Error())
	}

	log.Println("starting up")
	clientID := viper.GetString("bot.app_id")

	token := os.Getenv("DISCORD_API_TOKEN")

	db, err := db.NewSqliteStore("./db")
	if err != nil {
		log.Fatalf("unable to start: %s", err.Error())
	}

	servers, err := db.GetAllServerConfigs()
	if err != nil {
		log.Fatalf("unable to start: %s", err.Error())
	}
	log.Println("got server configs")

	for _, s := range servers {
		s.GuildChan = make(chan *discordgo.GuildCreate)
		s.MembersChan = make(chan *discordgo.GuildMembersChunk)
	}

	dg, err := InitializeDG(servers, token)
	if err != nil {
		log.Fatalf("unable to initialize discordgo client: %s", err.Error())
	}
	log.Println("dg initialized")

	for _, s := range servers {
		go setOffServerProcessing(dg.DG, s)
	}
	log.Println("Listening on :8080")
	http.HandleFunc("/link", server.ServeLinkPageFunc(clientID))
	http.ListenAndServe(":8080", nil)
}

func InitializeDG(servers []*models.Server, token string) (*models.GlobalConfig, error) {
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

func GuildMembersChunkFunc(servers []*models.Server) func(_ *discordgo.Session, gm *discordgo.GuildMembersChunk) {
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

func GuildCreateHandlerFunc(servers []*models.Server) func(_ *discordgo.Session, gc *discordgo.GuildCreate) {
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

// lookupUserRoles returns a slice of ids for roles a user has
func lookupUserRoles(mc *discordgo.GuildMembersChunk, UserID string) []string {
	for _, m := range mc.Members {
		if UserID == m.User.ID {
			return m.Roles
		}
	}
	return []string{}
}

func numberFromChannelName(prefix string, fullname string) (int, error) {
	if len(prefix) > len(fullname) {
		return 0, fmt.Errorf("this aint it, cuz")
	}
	numberMaybe := strings.Trim(fullname[len(prefix):], " ")
	return strconv.Atoi(numberMaybe)
}

func setOffServerProcessing(dg *discordgo.Session, seerver *models.Server) {
	fmt.Println("processing + ", seerver.ID)
	err := dg.RequestGuildMembers(seerver.ID, "", 0, "", true)
	if err != nil {
		log.Printf("error requesting guild members for server %s: %s", seerver.ID, err.Error())
	}
	// add the guild struct to the server once it comes in from the guildCreate channel
	guild := <-seerver.GuildChan

	// map of channelIDs to focus rooms
	focusRooms := make(map[string]*models.FocusRoom)
	for _, c := range guild.Channels {
		if c.Type == discordgo.ChannelTypeGuildVoice {
			if strings.HasPrefix(c.Name, seerver.ChannelPrefix) {
				number, err := numberFromChannelName(seerver.ChannelPrefix, c.Name)
				if err != nil {
					log.Printf("Unable to get channel number from channel name %s: %s", c.Name, err.Error())
				}
				// check for corresponding role
				var targetRole *discordgo.Role
				for _, role := range guild.Roles {
					rolenumber, err := numberFromChannelName(seerver.RolePrefix, role.Name)
					if err != nil {
						// probably not the role we're looking for
						continue
					} else if rolenumber == number {
						targetRole = role
						break
					}
					fmt.Println(rolenumber)
				}
				focusRooms[c.ID] = &models.FocusRoom{
					ChannelStruct: c,
					Users:         []string{},
					Number:        number,
					Role:          targetRole,
				}
			}
		}
	}

	fmt.Println("looping voice states")
	for _, voice_state := range guild.VoiceStates {
		targetCage, ok := focusRooms[voice_state.ChannelID]
		if !ok {
			// ignore because they're not in a vc we care about
			continue
		}
		targetCage.Users = append(targetCage.Users, voice_state.UserID)
	}

	fmt.Println("Looping focus rooms")
	for _, wc := range focusRooms {
		// if there's no one in them, delete the empties (and roles) save for the first one
		if len(wc.Users) == 0 && wc.Number != 0 {
			wc.Delete = true
		}
	}

	// get the lowest numbered wage cage and don't delete it
	var lowest *models.FocusRoom

	for _, wc := range focusRooms {
		if len(wc.Users) != 0 {
			continue
		}
		if lowest == nil {
			lowest = wc
		} else {
			if wc.Number < lowest.Number {
				lowest = wc
			}
		}
		wc.Delete = true
	}

	if lowest != nil {
		lowest.Delete = false
	}

	// delete all marked for deletion
	remainingWagecages := make(map[string]*models.FocusRoom)
	for _, wc := range focusRooms {
		if wc.Delete {
			_, err := dg.ChannelDelete(wc.ChannelStruct.ID)
			if err != nil {
				log.Printf("unable to delete channel with id %s: %s\n", wc.ChannelStruct.ID, err.Error())
			}
			if wc.Role != nil {
				err = dg.GuildRoleDelete(guild.ID, wc.Role.ID)
				if err != nil {
					log.Printf("unable to delete role with id %s: %s\n", wc.ChannelStruct.ID, err.Error())
				}
			}
		} else {
			remainingWagecages[wc.ChannelStruct.ID] = wc
		}
	}
	focusRooms = remainingWagecages

	// if all are filled up (also figure out the lowest unused number)
	createNew := true

	for _, wc := range focusRooms {
		if len(wc.Users) == 0 {
			createNew = false
			break
		}
	}

	// create another wage cage
	// create role as well (though this should probably be created immediately prior to giving it out
	if createNew {
		// select the lowest unused number here
		arr := make([]bool, len(focusRooms))
		for _, wc := range focusRooms {
			if wc.Number >= len(focusRooms) {
				continue
			} else {
				arr[wc.Number] = true
			}
		}

		var newNumber = len(focusRooms)
		for i, exists := range arr {
			if !exists {
				newNumber = i
				break
			}
		}
		var color = 69
		var hoist = false
		var mentionable = false
		var perms = int64(0)
		role, err := dg.GuildRoleCreate(guild.ID, &discordgo.RoleParams{
			Name:        seerver.RolePrefix + strconv.Itoa(newNumber),
			Color:       &color,
			Hoist:       &hoist,
			Permissions: &perms,
			Mentionable: &mentionable,
		})
		if err != nil {
			log.Printf("unable to create role: %s", err.Error())
		}
		//todo: set up something to listn for the creation to be confirmed and act on it instead of sleeping
		time.Sleep(time.Second * 1)
		channel, err := dg.GuildChannelCreate(guild.ID, seerver.ChannelPrefix+strconv.Itoa(newNumber), discordgo.ChannelTypeGuildVoice)
		if err != nil {
			log.Printf("unable to create new channel: %s ", err.Error())
		}

		time.Sleep(time.Second * 1)

		deny := int64(0)
		allow := int64(16777472)
		err = dg.ChannelPermissionSet(channel.ID, role.ID, discordgo.PermissionOverwriteTypeRole, allow, deny)
		if err != nil {
			log.Printf("unable to set perms on new channel: %s", err.Error())
		}
		focusRooms[channel.ID] = &models.FocusRoom{
			ChannelStruct: channel,
			Role:          role,
			Users:         []string{},
			Number:        newNumber,
			Delete:        false,
		}

		// add the users struct to the server for lookups once it comes from the members request above
		memberstore := <-seerver.MembersChan
		for _, wc := range focusRooms {
			if wc.Role == nil {
				continue
			}
			fmt.Printf("wc %d, usercount %d, role %s\n", wc.Number, len(wc.Users), wc.Role.Name)
			userfound := false
		searchForUserWithRole:
			for _, user := range wc.Users {
				fmt.Printf("%#v", user)
				for _, role := range lookupUserRoles(memberstore, user) {
					if role == wc.Role.ID {
						userfound = true
						break searchForUserWithRole
					}
				}
			}
			if !userfound && len(wc.Users) > 0 {
				//give user[0] the role
				err := dg.GuildMemberRoleAdd(guild.ID, wc.Users[0], wc.Role.ID)
				if err != nil {
					log.Printf("unable to add role to guild member: %s", err.Error())
				}
			}
		}

		// remove the roles if the user isn't in the wagecage for their role
		for _, m := range memberstore.Members {
			for _, wc := range focusRooms {
				found := false
				for _, user := range wc.Users {
					if m.User.ID == user {
						found = true
					}
				}
				if !found {
					for _, r := range m.Roles {
						if r == wc.Role.ID {
							err := dg.GuildMemberRoleRemove(GUILD_ID, m.User.ID, wc.Role.ID)
							if err != nil {
								log.Println(err.Error())
							}
						}
					}
				}
			}
		}
	}
}
