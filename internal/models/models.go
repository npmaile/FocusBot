package models

import "github.com/bwmarrin/discordgo"

type FocusRoom struct {
	ChannelStruct  *discordgo.Channel
	Role           *discordgo.Role
	Users          []string
	Number         int
	MarkDelete     bool
	MarkRoleDelete bool
}

type GuildConfig struct {
	ID              string
	ChannelPrefix   string
	RolePrefix      string
	ChannelCategory string
}

func DefaultGuildConfig(id string)*GuildConfig{
	return &GuildConfig{
		ID:              id,
		ChannelPrefix:   "FocusRoom",
		RolePrefix:      "Focus King",
		ChannelCategory: "",
	}
}

type GlobalConfig struct {
	Ready *discordgo.Ready
	// global dg session object
	DG *discordgo.Session
}
