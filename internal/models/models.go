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
}

func DefaultGuildConfig(id string) GuildConfig {
	return GuildConfig{
		ID:              id,
		ChannelPrefix:   "focus room",
		RolePrefix:      "focus owner",
	}

}

type GlobalConfig struct {
	Ready *discordgo.Ready
	// global dg session object
	DG *discordgo.Session
}
