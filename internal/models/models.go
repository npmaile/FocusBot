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
	ID            string
	ChannelPrefix string
	RolePrefix    string
}

type GlobalConfig struct {
	Ready *discordgo.Ready
	// global dg session object
	DG *discordgo.Session
}
