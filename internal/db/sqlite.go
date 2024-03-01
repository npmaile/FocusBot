package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/npmaile/focusbot/internal/models"
	"github.com/npmaile/focusbot/pkg/logerooni"
)

type sqliteStore struct {
	storage *sql.DB
}

// todo: use proper logging once it exists
func NewSqliteStore(filePath string) (DataStore, error) {
	logerooni.Debug("NewSqliteStore called")
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

func (s *sqliteStore) GetStupidDBIntersect(ids []string) ([]*models.GuildConfig, error) {
	logerooni.Debug("GetStupidDBIntersect called")
	dynamicPart := strings.Builder{}
	interslice := []interface{}{}
	for _, id := range ids {
		dynamicPart.WriteString("?,")
		interslice = append(interslice, id)
	}
	serversResponse, err := s.storage.Query(fmt.Sprintf(`SELECT
	id, channelPrefix, RolePrefix
	FROM
	servers
	WHERE
	id in (%s)
	;`, dynamicPart.String()[0:len(dynamicPart.String())-1]), interslice...)
	if err != nil {
		logerooni.Errorf("didn't work: %s", err.Error())
		panic("didn't work")
	}

	var ret []*models.GuildConfig
	for serversResponse.Next() {
		next := models.GuildConfig{}
		serversResponse.Scan(&next.ID, &next.ChannelPrefix, &next.RolePrefix)
		ret = append(ret, &next)
	}
	return ret, nil
}

func (s *sqliteStore) GetServerConfiguration(guildID string) (models.GuildConfig, error) {
	logerooni.Debug("GetServerConfiguration called")
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
	logerooni.Debug("GetAllServerConfigs called")
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
		logerooni.Debug(fmt.Sprintf("loaded config for guild: %s", guild.ID))
	}
	return ret, nil

}

func (s *sqliteStore) AddServer(cfg models.GuildConfig) error {
	_, err := s.storage.Exec(`INSERT 
		INTO servers (id, channelPrefix, rolePrefix)
		values (?,?,?)
	`, cfg.ID, cfg.ChannelPrefix, cfg.RolePrefix)
	if err != nil {
		logerooni.Errorf("unable to insert new entry to sqlite data store: %s", err.Error())
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
		logerooni.Errorf("unable to update entry to sqlite data store: %s", err.Error())
		return err
	}
	return nil
}
