package db

import (
	"github.com/npmaile/wagebot/internal/models"
)

type DataStore interface {
	GetServerConfiguration(guildID string) (models.GuildConfig, error)
	GetAllServerConfigs() ([]*models.GuildConfig, error)
	// todo: add server config updates to insert into database from management interface
}

// todo: add postgres db backend option
