package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	APPLICATION_ID = 1169055366085087312
	PUBLIC_KEY     = 0x54b745e43fd79dd0e3d8f494c1d12625adaa516af6551da56ca62c220bcde863
	PERMS_INTEGER  = 8
	GUILD_ID       = "1157158258621042799"
)

type wageCage struct {
	channelStruct *discordgo.Channel
	role          *discordgo.Role
	users         []*discordgo.Member
	number        int
	delete        bool
}

func main() {
	token := os.Getenv("DISCORD_API_TOKEN")

	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal("unable to initialize a new discordgo client: " + err.Error())
	}

	dg.StateEnabled = true
	dg.State.TrackChannels = true
	dg.State.TrackMembers = true
	dg.State.TrackRoles = true
	dg.State.TrackVoice = true
	dg.State.TrackPresences = true

	readychan := make(chan struct{})
	dg.AddHandler(func(_ *discordgo.Session, _ *discordgo.Ready) {
		readychan <- struct{}{}
	})
	err = dg.Open()
	if err != nil {
		log.Fatalf("error opening ws connection: %s", err.Error())
	}
	defer dg.Close()

	<-readychan

	guild := dg.State.Guilds[0]
	// TODO: wait for an event showing this is working instead of sleeping
	time.Sleep(1 * time.Second)

	wagecages := make(map[string]*wageCage)

	channelprefix := "WAGE CAGE #"
	roleprefix := "WAGE CAGE KING "

	for _, c := range guild.Channels {
		if c.Type == discordgo.ChannelTypeGuildVoice {
			if strings.HasPrefix(c.Name, channelprefix) {
				number, err := numberFromChannelName(channelprefix, c.Name)
				if err != nil {
					log.Printf("Unable to get channel number from channel name %s: %s", c.Name, err.Error())
				}
				// check for corresponding role
				var targetRole *discordgo.Role
				for _, role := range guild.Roles {
					rolenumber, err := numberFromChannelName(channelprefix, roleprefix)
					if err != nil {
						// probably not the role we're looking for
						continue
					} else if rolenumber == number {
						targetRole = role
						break
					}
				}
				wagecages[c.ID] = &wageCage{
					channelStruct: c,
					users:         []*discordgo.Member{},
					number:        number,
					role:          targetRole,
				}
			}
		}
	}

	for _, voice_state := range guild.VoiceStates {
		user := voice_state.Member
		//TODO: this doesn't work. Fix this somehow. listening for userUpdate and MemberUpdate didn't seem to work
		fmt.Println(user)
		targetCage, ok := wagecages[voice_state.ChannelID]
		if !ok {
			// ignore because they're not in a vc we care about
			continue
		}
		targetCage.users = append(targetCage.users, user)
	}

	for _, wc := range wagecages {
		// if there's no one in them, delete the empties (and roles) save for the first one
		if len(wc.users) == 0 && wc.number != 0 {
			wc.delete = true
		}
	}

	// get the lowest numbered wage cage and don't delete it
	var lowest *wageCage

	for _, wc := range wagecages {
		if len(wc.users) != 0 {
			continue
		}
		if lowest == nil {
			lowest = wc
		} else {
			if wc.number < lowest.number {
				lowest = wc
			}
		}
		wc.delete = true
	}
	/*
		for _, wc := range wagecages {
			fmt.Printf("channel %s marked for deletion", wc.channelStruct.Name)
		}
	*/

	if lowest != nil {
		lowest.delete = false
	}

	// delete all marked for deletion
	remainingWagecages := make(map[string]*wageCage)
	for _, wc := range wagecages {
		if wc.delete {
			_, err := dg.ChannelDelete(wc.channelStruct.ID)
			if err != nil {
				log.Printf("unable to delete channel with id %s: %s\n", wc.channelStruct.ID, err.Error())
			}
			if wc.role != nil {
				err = dg.GuildRoleDelete(guild.ID, wc.role.ID)
				if err != nil {
					log.Printf("unable to delete role with id %s: %s\n", wc.channelStruct.ID, err.Error())
				}
			}
		} else {
			remainingWagecages[wc.channelStruct.ID] = wc
		}
	}
	wagecages = remainingWagecages

	// if all are filled up (also figure out the lowest unused number)
	createNew := true

	for _, wc := range wagecages {
		if len(wc.users) == 0 {
			createNew = false
			break
		}
	}

	// create another wage cage
	// create role as well (though this should probably be created immediately prior to giving it out
	if createNew {
		// select the lowest unused number here
		arr := make([]bool, len(wagecages))
		for _, wc := range wagecages {
			if wc.number >= len(wagecages) {
				continue
			} else {
				arr[wc.number] = true
			}
		}

		var newNumber = len(wagecages)
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
			Name:        roleprefix + strconv.Itoa(newNumber),
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
		channel, err := dg.GuildChannelCreate(guild.ID, channelprefix+strconv.Itoa(newNumber), discordgo.ChannelTypeGuildVoice)
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
		wagecages[channel.ID] = &wageCage{
			channelStruct: channel,
			role:          role,
			users:         []*discordgo.Member{},
			number:        newNumber,
			delete:        false,
		}
	}

	for _, wc := range wagecages {
		userfound := false
	searchForUserWithRole:
		for _, user := range wc.users {
			fmt.Printf("%#v", user)
			for _, role := range user.Roles {
				if role == wc.role.ID {
					userfound = true
					break searchForUserWithRole
				}
			}
		}
		if !userfound && len(wc.users) > 0 {
			err := dg.GuildMemberRoleAdd(guild.ID, wc.users[0].User.ID, wc.role.ID)
			if err != nil {
				log.Printf("unable to add role to guild member: %s", err.Error())
			}
			//give user[0] the role
		}
	}
	// delete the roles if the user isn't in the wagecage for their role

}

func numberFromChannelName(prefix string, fullname string) (int, error) {
	numberMaybe := strings.Trim(fullname[len(prefix):], " ")
	return strconv.Atoi(numberMaybe)
}
