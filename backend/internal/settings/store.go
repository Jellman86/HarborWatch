package settings

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Settings struct {
	DiscordWebhookURL string `json:"discordWebhookUrl"`
	GotifyURL          string `json:"gotifyUrl"`
	GotifyToken        string `json:"gotifyToken"`
}

type Store struct {
	db *sql.DB
}

func OpenStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) GetDB() *sql.DB { return s.db }

func (s *Store) Init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS app_settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`)
	return err
}

func (s *Store) Get(ctx context.Context) (Settings, error) {
	var st Settings
	rows, err := s.db.QueryContext(ctx, "SELECT key, value FROM app_settings")
	if err != nil {
		return st, err
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return st, err
		}
		switch key {
		case "discord_webhook_url":
			st.DiscordWebhookURL = value
		case "gotify_url":
			st.GotifyURL = value
		case "gotify_token":
			st.GotifyToken = value
		}
	}
	return st, nil
}

func (s *Store) Save(ctx context.Context, st Settings) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	keys := map[string]string{
		"discord_webhook_url": st.DiscordWebhookURL,
		"gotify_url":          st.GotifyURL,
		"gotify_token":        st.GotifyToken,
	}

	for k, v := range keys {
		_, err = tx.ExecContext(ctx, `
INSERT INTO app_settings(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value
`, k, v)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
