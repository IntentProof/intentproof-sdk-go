package intentproof

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	_ "modernc.org/sqlite"
)

// Outbox stores signed execution events in SQLite WAL mode.
type Outbox struct {
	db   *sql.DB
	lock sync.Mutex
}

// OpenOutbox opens or creates the SQLite outbox at dbPath.
func OpenOutbox(dbPath string) (*Outbox, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("intentproof: open outbox: %w", err)
	}
	db.SetMaxOpenConns(1)
	o := &Outbox{db: db}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, err
	}
	schema := `
CREATE TABLE IF NOT EXISTS events (
    event_id TEXT PRIMARY KEY,
    body JSON NOT NULL
);
CREATE TABLE IF NOT EXISTS chains (
    correlation_id TEXT PRIMARY KEY,
    last_position INTEGER NOT NULL,
    last_hash TEXT NOT NULL
);`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return o, nil
}

func (o *Outbox) nextChainLink(correlationID string) (int, string, error) {
	var pos int
	var hash string
	err := o.db.QueryRow(
		"SELECT last_position, last_hash FROM chains WHERE correlation_id = ?",
		correlationID,
	).Scan(&pos, &hash)
	if err == sql.ErrNoRows {
		return 1, SentinelPrevHash, nil
	}
	if err != nil {
		return 0, "", err
	}
	return pos + 1, hash, nil
}

// RecordChainedEvent reserves a chain slot, signs, and persists atomically.
func (o *Outbox) RecordChainedEvent(
	correlationID, eventID string,
	buildSigned func(chainPos int, prevHash string) (map[string]any, string, error),
) (map[string]any, error) {
	o.lock.Lock()
	defer o.lock.Unlock()
	chainPos, prevHash, err := o.nextChainLink(correlationID)
	if err != nil {
		return nil, err
	}
	signed, eventHash, err := buildSigned(chainPos, prevHash)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(signed)
	if err != nil {
		return nil, err
	}
	tx, err := o.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		"INSERT INTO events (event_id, body) VALUES (?, ?)",
		eventID, string(body),
	); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`
INSERT INTO chains (correlation_id, last_position, last_hash)
VALUES (?, ?, ?)
ON CONFLICT(correlation_id) DO UPDATE SET
    last_position = excluded.last_position,
    last_hash = excluded.last_hash`,
		correlationID, chainPos, eventHash,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return signed, nil
}

// Events returns all persisted events.
func (o *Outbox) Events() ([]map[string]any, error) {
	o.lock.Lock()
	defer o.lock.Unlock()
	rows, err := o.db.Query("SELECT body FROM events")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var body string
		if err := rows.Scan(&body); err != nil {
			return nil, err
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(body), &ev); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// Close closes the database handle.
func (o *Outbox) Close() error {
	o.lock.Lock()
	defer o.lock.Unlock()
	return o.db.Close()
}
