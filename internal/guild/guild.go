package guild

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/npmaile/wagebot/internal/models"
)

func NewFromConfig(c models.GuildConfig) *Guild {
	return &Guild{
		GuildChan:        make(chan *discordgo.GuildCreate),
		MembersChan:      make(chan *discordgo.GuildMembersChunk),
		VoiceStateUpdate: make(chan *discordgo.VoiceStateUpdate),
		ID:               c.ID,
		ChannelPrefix:    c.ChannelPrefix,
		RolePrefix:       c.RolePrefix,
	}
}

type Guild struct {
	DgGuild          *discordgo.Guild
	GuildChan        chan *discordgo.GuildCreate
	MembersChan      chan *discordgo.GuildMembersChunk
	VoiceStateUpdate chan *discordgo.VoiceStateUpdate
	ID               string
	ChannelPrefix    string
	RolePrefix       string
}

func (server Guild) SetOffServerProcessing(dg *discordgo.Session) {
	fmt.Println("processing + ", server.ID)
	err := dg.RequestGuildMembers(server.ID, "", 0, "", true)
	if err != nil {
		log.Printf("error requesting guild members for server %s: %s", server.ID, err.Error())
	}
	// add the guild struct to the server once it comes in from the guildCreate channel
	guild := <-server.GuildChan

	// map of channelIDs to focus rooms
	focusRooms := make(map[string]*models.FocusRoom)
	for _, c := range guild.Channels {
		if c.Type == discordgo.ChannelTypeGuildVoice {
			if strings.HasPrefix(c.Name, server.ChannelPrefix) {
				number, err := numberFromChannelName(server.ChannelPrefix, c.Name)
				if err != nil {
					log.Printf("Unable to get channel number from channel name %s: %s", c.Name, err.Error())
				}
				// check for corresponding role
				var targetRole *discordgo.Role
				for _, role := range guild.Roles {
					rolenumber, err := numberFromChannelName(server.RolePrefix, role.Name)
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
	remainingFocusRooms := make(map[string]*models.FocusRoom)
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
			remainingFocusRooms[wc.ChannelStruct.ID] = wc
		}
	}
	focusRooms = remainingFocusRooms

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
			Name:        server.RolePrefix + strconv.Itoa(newNumber),
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
		channel, err := dg.GuildChannelCreate(guild.ID, server.ChannelPrefix+strconv.Itoa(newNumber), discordgo.ChannelTypeGuildVoice)
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
		memberstore := <-server.MembersChan
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
						if wc.Role == nil || r == wc.Role.ID {
							err := dg.GuildMemberRoleRemove(server.ID, m.User.ID, wc.Role.ID)
							if err != nil {
								log.Println(err.Error())
							}
						}
					}
				}
			}
		}
	}
	for {
		voiceUpdate := <-server.VoiceStateUpdate
		if voiceUpdate.ChannelID == "" {
			// user has left

			// check user's roles
			// if any of them are the wage channel ones, get rid of it
			//err := dg.GuildMemberRoleRemove(server.ID, voiceUpdate.UserID, room.Role.ID)
			/*	if err != nil {
				//todo: fix later
				fmt.Println("error returned")
			}*/
		} else {
			for id, room := range focusRooms {
				if id == voiceUpdate.ChannelID {

				}
			}
			//user has joined
			// check roles of users in channel
			// if no one has the role in the channel, add the role to this fella
		}

	}

}

func numberFromChannelName(prefix string, fullname string) (int, error) {
	if len(prefix) > len(fullname) {
		return 0, fmt.Errorf("this aint it, cuz")
	}
	numberMaybe := strings.Trim(fullname[len(prefix):], " ")
	return strconv.Atoi(numberMaybe)
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
