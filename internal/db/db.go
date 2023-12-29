package db

import (
	"github.com/npmaile/wagebot/internal/models"
)

type DataStore interface {
	GetServerConfiguration(guildID string) (models.GuildConfig, error)
	GetAllServerConfigs() ([]*models.GuildConfig, error)
	AddServerConfiguration(*models.GuildConfig) error
	UpdateServerConfiguration(*models.GuildConfig) error
}

// todo: add postgres db backend option
