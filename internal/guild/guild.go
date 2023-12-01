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

func NewFromConfig(c *models.GuildConfig) *Guild {
	return &Guild{
		GuildChan:        make(chan *discordgo.GuildCreate),
		MembersChan:      make(chan *discordgo.GuildMembersChunk),
		VoiceStateUpdate: make(chan *discordgo.VoiceStateUpdate),
		Config:           c,
	}
}

type Guild struct {
	GuildChan        chan *discordgo.GuildCreate
	MembersChan      chan *discordgo.GuildMembersChunk
	VoiceStateUpdate chan *discordgo.VoiceStateUpdate
	focusRooms       map[string]*models.FocusRoom
	Config           *models.GuildConfig
	Initialized      bool
}

func (server *Guild) getServerStateInTheRightPlace(dg *discordgo.Session) {
	err := dg.RequestGuildMembers(server.Config.ID, "", 0, "", true)
	if err != nil {
		log.Printf("error requesting guild members for server %s: %s", server.Config.ID, err.Error())
	}
	// add the guild struct to the server once it comes in from the guildCreate channel
	if !server.Initialized {
		<-server.GuildChan
		server.Initialized = true
	}

	var guild *discordgo.Guild
	for _, g := range dg.State.Guilds {
		if g.ID == server.Config.ID {
			guild = g
			break
		}
	}
	// map of channelIDs to focus rooms
	focusRooms := make(map[string]*models.FocusRoom)
	for _, c := range guild.Channels {
		if c.Type == discordgo.ChannelTypeGuildVoice {
			if strings.HasPrefix(c.Name, server.Config.ChannelPrefix) {
				number, err := numberFromChannelName(server.Config.ChannelPrefix, c.Name)
				if err != nil {
					log.Printf("Unable to get channel number from channel name %s: %s", c.Name, err.Error())
				}
				// check for corresponding role
				var targetRole *discordgo.Role
				for _, role := range guild.Roles {
					rolenumber, err := numberFromChannelName(server.Config.RolePrefix, role.Name)
					if err != nil {
						// probably not the role we're looking for
						continue
					} else if rolenumber == number {
						targetRole = role
						break
					}
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

	for _, voice_state := range guild.VoiceStates {
		targetCage, ok := focusRooms[voice_state.ChannelID]
		if !ok {
			// ignore because they're not in a vc we care about
			continue
		}
		targetCage.Users = append(targetCage.Users, voice_state.UserID)
	}

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
			delete(focusRooms, wc.ChannelStruct.ID)
		}
	}

	// if all are filled up (also figure out the lowest unused number)
	createNew := true

	for _, wc := range focusRooms {
		if len(wc.Users) == 0 {
			createNew = false
			break
		}
	}

	channelParentID := ""
	for _, ch := range guild.Channels {
		if ch.Type == discordgo.ChannelTypeGuildCategory && strings.EqualFold(ch.Name, server.Config.ChannelCategory) {
			channelParentID = ch.ID
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
			Name:        server.Config.RolePrefix + strconv.Itoa(newNumber),
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
		channel, err := dg.GuildChannelCreateComplex(guild.ID, discordgo.GuildChannelCreateData{
			Name:     server.Config.ChannelPrefix + strconv.Itoa(newNumber),
			Type:     discordgo.ChannelTypeGuildVoice,
			Topic:    "Focus",
			ParentID: channelParentID,
		})
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
							err := dg.GuildMemberRoleRemove(server.Config.ID, m.User.ID, wc.Role.ID)
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

func (server Guild) SetOffServerProcessing(dg *discordgo.Session) {
	server.getServerStateInTheRightPlace(dg)
	for {
		<-server.VoiceStateUpdate
		server.getServerStateInTheRightPlace(dg)
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
