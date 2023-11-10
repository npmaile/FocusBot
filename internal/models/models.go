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
	ID            string
	ChannelPrefix string
	RolePrefix    string
}
