package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/npmaile/wagebot/internal/models"
)


type sqliteStore struct {
	storage *sql.DB
}

//  todo: use proper logging once it exists
func NewSqliteStore(filePath string) (DataStore, error) {
	// add the filepath to store the databe to the config toml
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to get new sqlite data store: %s", err.Error())
	}
	_, err = db.Exec(`CREATE TABLE if not exists 
	servers(id TEXT, channelPrefix TEXT, rolePrefix TEXT, channelCategory TEXT)`)
	if err != nil {
		return nil, fmt.Errorf("unable to get new sqlite data store: %s", err.Error())
	}
	return &sqliteStore{
		storage: db,
	}, nil
}

func (s *sqliteStore) GetServerConfiguration(guildID string) (models.GuildConfig, error) {
	row := s.storage.QueryRow(`SELECT
	id, channelPrefix, rolePrefix, channelCategory
	FROM
	servers
	WHERE
	id = ?`, guildID)
	err := row.Err()
	if err != nil {
		return models.GuildConfig{}, fmt.Errorf("unable to get server configuration: %s", err.Error())
	}
	var ret models.GuildConfig
	err = row.Scan(&ret.ID, &ret.ChannelPrefix, &ret.RolePrefix, &ret.ChannelCategory)
	if err != nil {
		return models.GuildConfig{}, fmt.Errorf("unable to get server configuration: %s", err.Error())
	}
	return ret, nil
}

func (s *sqliteStore) GetAllServerConfigs() ([]*models.GuildConfig, error) {
	rows, err := s.storage.Query(`SELECT
	id, channelPrefix, rolePrefix, channelCategory
	FROM
	servers`)
	if err != nil {
		return nil, fmt.Errorf("unable to list server configurations: %s", err.Error())
	}
	var ret []*models.GuildConfig
	for rows.Next() {
		s := models.GuildConfig{}
		err := rows.Scan(&s.ID, &s.ChannelPrefix, &s.RolePrefix, &s.ChannelCategory)
		if err != nil {
			return nil, fmt.Errorf("unable to scan server configs into struct: %s", err.Error())
		}
		ret = append(ret, &s)
	}
	return ret, nil

}
