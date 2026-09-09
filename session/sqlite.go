package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// InMemoryStore is a single-session, in-process Store, a port of pymax's
// session.store.InMemoryStore. Used when persistence is disabled.
type InMemoryStore struct {
	mu      sync.Mutex
	session *Info
}

// NewInMemoryStore builds an empty in-memory session store.
func NewInMemoryStore() *InMemoryStore { return &InMemoryStore{} }

func (s *InMemoryStore) SaveSession(ctx context.Context, info Info) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := info
	s.session = &cp
	return nil
}

func (s *InMemoryStore) UpdateToken(ctx context.Context, oldToken, newToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return nil
	}
	s.session.Token = newToken
	return nil
}

func (s *InMemoryStore) LoadSession(ctx context.Context) (*Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return nil, nil
	}
	cp := *s.session
	return &cp, nil
}

func (s *InMemoryStore) LoadSessionByDeviceID(ctx context.Context, deviceID string) (*Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil || s.session.DeviceID != deviceID {
		return nil, nil
	}
	cp := *s.session
	return &cp, nil
}

func (s *InMemoryStore) LoadSessionByPhone(ctx context.Context, phone string) (*Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil || s.session.Phone != phone {
		return nil, nil
	}
	cp := *s.session
	return &cp, nil
}

func (s *InMemoryStore) DeleteSession(ctx context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.session = nil
	return nil
}

func (s *InMemoryStore) Close(ctx context.Context) error { return nil }

// SQLiteStore is the default SQLite-backed Store, a port of pymax's
// session.store.SessionStore. It uses modernc.org/sqlite, a pure-Go driver,
// so the module does not require cgo.
type SQLiteStore struct {
	dbPath string

	mu sync.Mutex
	db *sql.DB
}

// NewSQLiteStore opens (creating if needed) a SQLite session store at
// workDir/dbName.
func NewSQLiteStore(workDir, dbName string) (*SQLiteStore, error) {
	if dbName == "" {
		dbName = "session.db"
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return nil, fmt.Errorf("session: create work dir: %w", err)
	}
	return &SQLiteStore{dbPath: filepath.Join(workDir, dbName)}, nil
}

func (s *SQLiteStore) connection() (*sql.DB, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		return s.db, nil
	}

	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return nil, err
	}
	if err := initSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	s.db = db
	return db, nil
}

func initSchema(db *sql.DB) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			token TEXT NOT NULL PRIMARY KEY,
			device_id TEXT NOT NULL,
			phone TEXT NOT NULL,
			mt_instance_id TEXT NOT NULL DEFAULT '',
			chats_sync INTEGER NOT NULL DEFAULT -1,
			contacts_sync INTEGER NOT NULL DEFAULT -1,
			drafts_sync INTEGER NOT NULL DEFAULT -1,
			presence_sync INTEGER NOT NULL DEFAULT -1,
			config_hash TEXT NOT NULL DEFAULT '',
			user_agent TEXT
		)
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`UPDATE sessions SET config_hash = ? WHERE config_hash = ''`, DefaultConfigHash); err != nil {
		return err
	}
	return nil
}

const sessionColumns = `token, device_id, phone, mt_instance_id, chats_sync, contacts_sync, drafts_sync, presence_sync, config_hash, user_agent`

func (s *SQLiteStore) SaveSession(ctx context.Context, info Info) error {
	db, err := s.connection()
	if err != nil {
		return err
	}

	var userAgentJSON any
	if info.UserAgent != nil {
		b, err := json.Marshal(info.UserAgent)
		if err != nil {
			return err
		}
		userAgentJSON = string(b)
	}

	_, err = db.ExecContext(ctx, `
		INSERT OR REPLACE INTO sessions (`+sessionColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		info.Token, info.DeviceID, info.Phone, info.MtInstanceID,
		info.Sync.ChatsSync, info.Sync.ContactsSync, info.Sync.DraftsSync, info.Sync.PresenceSync,
		info.Sync.ConfigHash, userAgentJSON,
	)
	return err
}

func (s *SQLiteStore) LoadSession(ctx context.Context) (*Info, error) {
	db, err := s.connection()
	if err != nil {
		return nil, err
	}
	row := db.QueryRowContext(ctx, `SELECT `+sessionColumns+` FROM sessions LIMIT 1`)
	return scanSession(row)
}

func (s *SQLiteStore) LoadSessionByDeviceID(ctx context.Context, deviceID string) (*Info, error) {
	db, err := s.connection()
	if err != nil {
		return nil, err
	}
	row := db.QueryRowContext(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE device_id = ?`, deviceID)
	return scanSession(row)
}

func (s *SQLiteStore) LoadSessionByPhone(ctx context.Context, phone string) (*Info, error) {
	db, err := s.connection()
	if err != nil {
		return nil, err
	}
	row := db.QueryRowContext(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE phone = ?`, phone)
	return scanSession(row)
}

func (s *SQLiteStore) DeleteSession(ctx context.Context, token string) error {
	db, err := s.connection()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// DeleteAllSessions removes every stored session, a port of pymax's
// session.store.SessionStore.delete_all_sessions.
func (s *SQLiteStore) DeleteAllSessions(ctx context.Context) error {
	db, err := s.connection()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `DELETE FROM sessions`)
	return err
}

func (s *SQLiteStore) UpdateToken(ctx context.Context, oldToken, newToken string) error {
	db, err := s.connection()
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `UPDATE sessions SET token = ? WHERE token = ?`, newToken, oldToken)
	return err
}

func (s *SQLiteStore) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func scanSession(row *sql.Row) (*Info, error) {
	var info Info
	var mtInstanceID, configHash sql.NullString
	var userAgentJSON sql.NullString

	err := row.Scan(
		&info.Token, &info.DeviceID, &info.Phone, &mtInstanceID,
		&info.Sync.ChatsSync, &info.Sync.ContactsSync, &info.Sync.DraftsSync, &info.Sync.PresenceSync,
		&configHash, &userAgentJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	info.MtInstanceID = mtInstanceID.String
	if configHash.Valid && configHash.String != "" {
		info.Sync.ConfigHash = configHash.String
	} else {
		info.Sync.ConfigHash = DefaultConfigHash
	}

	if userAgentJSON.Valid {
		var ua UserAgent
		if err := json.Unmarshal([]byte(userAgentJSON.String), &ua); err != nil {
			return nil, err
		}
		info.UserAgent = &ua
	}

	return &info, nil
}
