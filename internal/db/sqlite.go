package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"github.com/npmaile/focusbot/internal/models"
	slog "github.com/npmaile/focusbot/pkg/logerooni"
)

type sqliteStore struct {
	storage *sql.DB
}

// todo: use proper logging once it exists
func NewSqliteStore(filePath string) (DataStore, error) {
	slog.Debug("NewSqliteStore called")
	// add the filepath to store the databe to the config toml
	db, err := sql.Open("sqlite3", filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to get new sqlite data store: %s", err.Error())
	}
	_, err = db.Exec(`CREATE TABLE if not exists 
	servers(id TEXT, channelPrefix TEXT, rolePrefix TEXT)`)
	if err != nil {
		return nil, fmt.Errorf("unable to get new sqlite data store: %s", err.Error())
	}
	return &sqliteStore{
		storage: db,
	}, nil
}

func (s *sqliteStore) GetServerConfiguration(guildID string) (models.GuildConfig, error) {
	slog.Debug("GetServerConfiguration called")
	row := s.storage.QueryRow(`SELECT
	id, channelPrefix, rolePrefix
	FROM
	servers
	WHERE
	id = ?`, guildID)
	err := row.Err()
	if err != nil {
		return models.GuildConfig{}, fmt.Errorf("unable to get server configuration: %s", err.Error())
	}
	var ret models.GuildConfig
	err = row.Scan(&ret.ID, &ret.ChannelPrefix, &ret.RolePrefix)
	if err != nil {
		return models.GuildConfig{}, fmt.Errorf("unable to get server configuration: %s", err.Error())
	}
	return ret, nil
}

func (s *sqliteStore) GetAllServerConfigs() ([]*models.GuildConfig, error) {
	slog.Debug("GetAllServerConfigs called")
	rows, err := s.storage.Query(`SELECT
	id, channelPrefix, rolePrefix
	FROM
	servers`)
	if err != nil {
		return nil, fmt.Errorf("unable to list server configurations: %s", err.Error())
	}
	var ret []*models.GuildConfig
	for rows.Next() {
		s := models.GuildConfig{}
		err := rows.Scan(&s.ID, &s.ChannelPrefix, &s.RolePrefix)
		if err != nil {
			return nil, fmt.Errorf("unable to scan server configs into struct: %s", err.Error())
		}
		ret = append(ret, &s)
	}
	for _, guild := range ret {
		slog.Debug(fmt.Sprintf("loaded config for guild: %s", guild.ID))
	}
	return ret, nil

}

func (s *sqliteStore) AddServer(cfg models.GuildConfig) error {
	_, err := s.storage.Exec(`INSERT 
		INTO servers (id, channelPrefix, rolePrefix)
		values (?,?,?)
	`, cfg.ID, cfg.ChannelPrefix, cfg.RolePrefix)
	if err != nil {
		slog.Errorf("unable to insert new entry to sqlite data store: %s", err.Error())
		return err
	}
	return nil
}

func (s *sqliteStore) UpdateServer(cfg models.GuildConfig) error {
	_, err := s.storage.Exec(`UPDATE	
		servers 
		SET
		channelPrefix = ?,
		rolePrefix = ?,
		WHERE
		id =?`, cfg.ChannelPrefix, cfg.RolePrefix)
	if err != nil {
		slog.Errorf("unable to update entry to sqlite data store: %s", err.Error())
		return err
	}
	return nil
}
