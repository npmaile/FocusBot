package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/npmaile/wagebot/internal/models"
)

type DataStore interface {
	GetServerConfiguration(guildID string) (models.Server, error)
}

type sqliteStore struct {
	storage *sql.DB
}

func NewSqliteStore(filePath string) (DataStore, error) {
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to get new sqlite data store: %s", err.Error())
	}
	return &sqliteStore{
		storage: db,
	}, nil
}

func (s *sqliteStore) GetServerConfiguration(guildID string) (models.Server, error) {
	row := s.storage.QueryRow(`SELECT
	id, channelPrefix, rolePrefix
	FROM
	servers
	WHERE
	id = ?`, guildID)
	err := row.Err()
	if err != nil {
		return models.Server{}, fmt.Errorf("unable to get server configuration: %s", err.Error())
	}
	var ret models.Server
	err = row.Scan(ret.ID, ret.ChannelPrefix, ret.RolePrefix)
	if err != nil {
		return models.Server{}, fmt.Errorf("unable to get server configuration: %s", err.Error())
	}
	return ret, nil
}
