package models

import "github.com/bwmarrin/discordgo"

type FocusRoom struct {
	ChannelStruct *discordgo.Channel
	Role          *discordgo.Role
	Users         []string
	Number        int
	Delete        bool
}

type Server struct {
	Guild         *discordgo.Guild
	GuildChan     chan *discordgo.GuildCreate
	MembersChan   chan *discordgo.GuildMembersChunk
	ID            string
	ChannelPrefix string
	RolePrefix    string
}

type GlobalConfig struct {
	Ready *discordgo.Ready
	// global dg session object
	DG *discordgo.Session
}
