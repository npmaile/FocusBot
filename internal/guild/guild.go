package guild

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/npmaile/focusbot/internal/models"
	"github.com/npmaile/focusbot/pkg/logerooni"
)

const staleMembersChunkTime = 2 * time.Second

func NewFromConfig(c *models.GuildConfig) *Guild {
	return &Guild{
		GuildChan:        make(chan *discordgo.GuildCreate),
		VoiceStateUpdate: make(chan *discordgo.VoiceStateUpdate),
		focusRooms:       map[string]*models.FocusRoom{},
		Config:           c,
		Members: membersAbstraction{
			timeUpdated: time.Time{},
			members:     map[string]*discordgo.Member{},
			MembersChan: make(chan *discordgo.GuildMembersChunk, 50),
			mtex:        sync.Mutex{},
		},
		Initialized: false,
	}
}

type Guild struct {
	GuildChan        chan *discordgo.GuildCreate
	VoiceStateUpdate chan *discordgo.VoiceStateUpdate
	focusRooms       map[string]*models.FocusRoom
	Config           *models.GuildConfig
	Members          membersAbstraction
	Initialized      bool
}

type membersAbstraction struct {
	timeUpdated time.Time
	members     map[string]*discordgo.Member
	MembersChan chan *discordgo.GuildMembersChunk
	mtex        sync.Mutex
}

func (m *membersAbstraction) WaitForSync() {
	for m.stale() {
		time.Sleep(5 * time.Millisecond)
	}
}

func (m *membersAbstraction) stale() bool {
	// this is deceptively difficult to reason about.
	m.mtex.Lock()
	ret := m.timeUpdated.Compare(time.Now().Add(-staleMembersChunkTime)) > 0
	m.mtex.Unlock()
	return ret
}

func (m *membersAbstraction) startReceivingMembersUpdates(){
	for {
		gmc := <- m.MembersChan
		m.updateMembers(gmc)
		m.timeUpdated = time.Now()
	}
}

func (m *membersAbstraction) updateMembers(gmc *discordgo.GuildMembersChunk) {
	m.mtex.Lock()
	for _, memb := range gmc.Members {
		m.members[memb.User.ID] = memb
	}
	m.mtex.Unlock()
}

func (m *membersAbstraction) getRoles(userID string) []string {
	m.mtex.Lock()
	user, found := m.members[userID]
	m.mtex.Unlock()
	if !found {
		return []string{}
	}
	return user.Roles
}

//todo: Currently it re-runs the initialization routine every time someone enters or leaves a channel. It should be more exact in what happens.

func (server *Guild) getServerStateInTheRightPlace(dg *discordgo.Session) {
	logerooni.Debug("inside of getServerStateInTheRightPlace")
	err := dg.RequestGuildMembers(server.Config.ID, "", 0, "", true)
	if err != nil {
		logerooni.Errorf("error requesting guild members for server %s: %s", server.Config.ID, err.Error())
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
	server.RefreshChannelState(dg, guild)

	// todo: abstract the following routine to it's own function
	logerooni.Debug("looping through voice states")
	for _, voice_state := range guild.VoiceStates {
		targetCage, ok := server.focusRooms[voice_state.ChannelID]
		if !ok {
			// ignore because they're not in a vc we care about
			continue
		}
		targetCage.Users = append(targetCage.Users, voice_state.UserID)
	}

	// todo: abstract the following routine to it's own function
	logerooni.Debug("marking rooms for deletion")
	for _, wc := range server.focusRooms {
		// if there's no one in them, delete the empties (and roles) save for the first one
		if len(wc.Users) == 0 && wc.Number != 0 {
			wc.MarkDelete = true
		}
	}

	// get the lowest numbered focus cage and don't delete it
	var lowest *models.FocusRoom

	// todo: abstract the following routine to it's own function
	logerooni.Debug("Pardoning one room")
	for _, wc := range server.focusRooms {
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
		wc.MarkDelete = true
	}
	if lowest != nil {
		lowest.MarkDelete = false
		lowest.MarkRoleDelete = true
	}

	// delete all marked for deletion
	// todo: abstract the following routines to it's own function
	for _, wc := range server.focusRooms {
		if wc.MarkDelete {
			logerooni.Debugf("deleting channel %s in server %s", wc.ChannelStruct.ID, server.Config.ID)
			_, err = dg.ChannelDelete(wc.ChannelStruct.ID)
			if err != nil {
				logerooni.Errorf("unable to delete channel with id %s: %s\n", wc.ChannelStruct.ID, err.Error())
			}
			if wc.Role != nil {
				err = dg.GuildRoleDelete(guild.ID, wc.Role.ID)
				if err != nil {
					logerooni.Errorf("unable to delete role with id %s: %s\n", wc.ChannelStruct.ID, err.Error())
				}
			}
			delete(server.focusRooms, wc.ChannelStruct.ID)
		} else if wc.MarkRoleDelete {
			logerooni.Debugf("deleting role %s in server %s", wc.Role.ID, server.Config.ID)
			if wc.Role != nil {
				err = dg.GuildRoleDelete(guild.ID, wc.Role.ID)
				if err != nil {
					logerooni.Errorf("unable to delete role with id %s: %s\n", wc.ChannelStruct.ID, err.Error())
				}
				wc.Role = nil
			}

		}
	}

	// if all are filled up (also figure out the lowest unused number)
	shouldCreateNew := true

	// todo: abstract the following routine to it's own function
	for _, wc := range server.focusRooms {
		if len(wc.Users) == 0 {
			shouldCreateNew = false
			break
		}
	}

	// create another focus cage
	// create role as well (though this should probably be created immediately prior to giving it out
	if shouldCreateNew {
		server.CreateNextChannel(dg)
	}

	// add the users struct to the server for lookups once it comes from the members request above
	// todo: abstract the following routine to it's own function
	logerooni.Debug("waiting for members chan\n")
	server.Members.WaitForSync()
	logerooni.Debug("got members chan\n")
	for _, wc := range server.focusRooms {
		server.AssignRole(dg, wc)
	}

}

func (server *Guild) AssignRole(dg *discordgo.Session, wc *models.FocusRoom) {
	logerooni.Debugf("AssignRole called for server %s, focusroom #%d usercount %d", server.Config.ID, wc.Number, len(wc.Users))
	if wc.Role == nil {
		wc.Role = server.CreateRole(dg, wc.Number)
		if wc.Role == nil {
			return
		}
	}
	if len(wc.Users) == 0 {
		return
	}
	userfound := false
searchForUserWithRole:
	for _, user := range wc.Users {
		for _, role := range server.Members.getRoles(user) {
			if role == wc.Role.ID {
				userfound = true
				break searchForUserWithRole
			}
		}
	}

	if !userfound {
		//give user[0] the role
		err := dg.GuildMemberRoleAdd(server.Config.ID, wc.Users[0], wc.Role.ID)
		if err != nil {
			logerooni.Errorf("unable to add role to guild member: %s", err.Error())
		} else {
			logerooni.Debugf("assigned role %s to user %s", wc.Role.ID, wc.Users[0])
		}

	}

}

// bug: roles not being given out
// bug: need to create role for channels missing roles
func (server *Guild) CreateNextChannel(dg *discordgo.Session) {
	logerooni.Debugf("CreateNextChannel called in server %s", server.Config.ID)
	// select the lowest unused number here
	arr := make([]bool, len(server.focusRooms))
	for _, wc := range server.focusRooms {
		if wc.Number >= len(server.focusRooms) {
			continue
		} else {
			arr[wc.Number] = true
		}
	}

	var newNumber = len(server.focusRooms)
	for i, exists := range arr {
		if !exists {
			newNumber = i
			break
		}
	}
	role := server.CreateRole(dg, newNumber)
	//todo: set up something to listen for the creation to be confirmed and act on it instead of sleeping
	logerooni.Debugf("Creating Channel number %d for server %s", newNumber, server.Config.ID)
	channel, err := dg.GuildChannelCreateComplex(server.Config.ID, discordgo.GuildChannelCreateData{
		Name:     server.Config.ChannelPrefix + strconv.Itoa(newNumber),
		Type:     discordgo.ChannelTypeGuildVoice,
		Topic:    "Focus",
		ParentID: server.GetRoomZeroCategory(),
	})
	if err != nil {
		logerooni.Errorf("unable to create new channel: %s ", err.Error())
	}

	deny := int64(0)
	allow := int64(16777472)
	err = dg.ChannelPermissionSet(channel.ID, role.ID, discordgo.PermissionOverwriteTypeRole, allow, deny)
	if err != nil {
		logerooni.Errorf("unable to set perms on new channel: %s", err.Error())
	}
	server.focusRooms[channel.ID] = &models.FocusRoom{
		ChannelStruct: channel,
		Role:          role,
		Users:         []string{},
		Number:        newNumber,
		MarkDelete:    false,
	}

}

func (server *Guild) GetRoomZeroCategory() string {
	logerooni.Debugf("asking for room category for channel zero in server %s", server.Config.ID)
	roomzero := ""
	for id, room := range server.focusRooms {
		if room.Number == 0 {
			roomzero = id
		}
	}
	if roomzero == "" {
		return ""
	}
	ret := server.focusRooms[roomzero].ChannelStruct.ParentID
	logerooni.Debugf("room index zero has parent %s in server %s", ret, server.Config.ID)
	return ret
}

func (server *Guild) CreateRole(dg *discordgo.Session, number int) *discordgo.Role {
	logerooni.Debugf("CreateRoleCalled for number %d in server %s", number, server.Config.ID)
	var color = 69
	var hoist = false
	var mentionable = false
	var perms = int64(0)
	role, err := dg.GuildRoleCreate(server.Config.ID, &discordgo.RoleParams{
		Name:        server.Config.RolePrefix + strconv.Itoa(number),
		Color:       &color,
		Hoist:       &hoist,
		Permissions: &perms,
		Mentionable: &mentionable,
	})
	if err != nil {
		logerooni.Errorf("unable to create role: %s", err.Error())
	} else {
		logerooni.Debugf("Created role %s in guild %s", role.Name, server.Config.ID)
	}
	return role
}

func (server *Guild) RefreshChannelState(dg *discordgo.Session, guild *discordgo.Guild) {
	logerooni.Debugf("RefreshChannelState called in guild %s", server.Config.ID)
	server.focusRooms = make(map[string]*models.FocusRoom)
	for _, c := range guild.Channels {
		if c.Type == discordgo.ChannelTypeGuildVoice && strings.HasPrefix(c.Name, server.Config.ChannelPrefix) {
			number, err := numberFromChannelName(server.Config.ChannelPrefix, c.Name)
			if err != nil {
				logerooni.Errorf("Unable to get channel number from channel name %s: %s", c.Name, err.Error())
			}
			// check for corresponding role
			logerooni.Debugf("checking for role associated with focus room %d in server %s", number, server.Config.ID)
			var targetRole *discordgo.Role
			for _, role := range guild.Roles {
				var rolenumber int
				rolenumber, err = numberFromChannelName(server.Config.RolePrefix, role.Name)
				if err != nil {
					// not the role we're looking for
					continue
				} else if rolenumber == number {
					targetRole = role
					break
				}
			}
			if targetRole == nil {
				server.CreateRole(dg, number)
			}
			server.focusRooms[c.ID] = &models.FocusRoom{
				ChannelStruct: c,
				Users:         []string{},
				Number:        number,
				Role:          targetRole,
			}
		}
	}
}

func (server *Guild) SetOffServerProcessing(dg *discordgo.Session) {
	logerooni.Debugf("Starting processing for guild %s", server.Config.ID)
	server.getServerStateInTheRightPlace(dg)
	for {
		vsu := <-server.VoiceStateUpdate
		logerooni.Debugf("voice state update receieved for user %s %s", vsu.UserID, deltaVoiceStateStatusBullshit(vsu))
		done := make(chan struct{})
		go func(done chan struct{}) {
			server.getServerStateInTheRightPlace(dg)
			done <- struct{}{}
		}(done)
		select {
		case <-time.After(10 * time.Second):
		case <-done:
		}
	}
}

func deltaVoiceStateStatusBullshit(vsu *discordgo.VoiceStateUpdate) string {
	if vsu.BeforeUpdate == nil {
		return fmt.Sprintf("joined %s", vsu.ChannelID)
	} else if vsu.ChannelID == "" {
		return fmt.Sprintf("left %s", vsu.BeforeUpdate.ChannelID)
	} else if vsu.BeforeUpdate.ChannelID != vsu.ChannelID {
		return fmt.Sprintf("left %s and joined %s", vsu.BeforeUpdate.ChannelID, vsu.ChannelID)
	}
	return "did nothing of consequence"
}

func numberFromChannelName(prefix string, fullname string) (int, error) {
	if len(prefix) > len(fullname) {
		return 0, fmt.Errorf("this aint it, cuz")
	}
	//todo: change to strings.trimprefix
	numberMaybe := strings.Trim(fullname[len(prefix):], " ")
	return strconv.Atoi(numberMaybe)
}

// lookupUserRoles returns a slice of ids for roles a user has
func lookupUserRoles(mc *discordgo.GuildMembersChunk, UserID string) []string {
	logerooni.Debugf("[lookupUserRoles] Looking up user roles for UserID %s", UserID)
	for _, m := range mc.Members {
		if UserID == m.User.ID {
			return m.Roles
		}
	}
	return []string{}
}
