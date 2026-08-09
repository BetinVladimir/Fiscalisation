package ota

import (
	"database/sql"
	"encoding/json"
	"errors"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct{ db *sql.DB }

func OpenSQLiteStore(path string, initial State) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{`pragma journal_mode=WAL`, `pragma synchronous=FULL`, `create table if not exists ota_state(singleton integer primary key check(singleton=1), revision integer not null, state_json blob not null)`} {
		if _, err = db.Exec(statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	if initial.Revision == 0 {
		initial.Revision = 1
	}
	body, err := json.Marshal(initial)
	if err != nil {
		db.Close()
		return nil, err
	}
	if _, err = db.Exec(`insert or ignore into ota_state(singleton,revision,state_json) values(1,?,?)`, initial.Revision, body); err != nil {
		db.Close()
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) Load() (State, error) {
	var body []byte
	var revision uint64
	if err := s.db.QueryRow(`select revision,state_json from ota_state where singleton=1`).Scan(&revision, &body); err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(body, &state); err != nil {
		return State{}, errors.New("corrupt OTA state")
	}
	if state.Revision != revision || revision == 0 {
		return State{}, errors.New("corrupt OTA revision")
	}
	return state, nil
}

func (s *SQLiteStore) Save(state *State) error {
	if state == nil || state.Revision == 0 {
		return errors.New("invalid OTA state revision")
	}
	expected := state.Revision
	state.Revision++
	body, err := json.Marshal(state)
	if err != nil {
		state.Revision = expected
		return err
	}
	result, err := s.db.Exec(`update ota_state set revision=?,state_json=? where singleton=1 and revision=?`, state.Revision, body, expected)
	if err != nil {
		state.Revision = expected
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		state.Revision = expected
		return errors.New("stale OTA state update rejected")
	}
	return nil
}
