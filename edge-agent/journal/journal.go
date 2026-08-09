package journal

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Event struct {
	Sequence       int64           `json:"sequence"`
	EventID        string          `json:"event_id"`
	OperationID    string          `json:"operation_id"`
	Type           string          `json:"type"`
	Payload        json.RawMessage `json:"payload"`
	PreviousHash   string          `json:"previous_hash,omitempty"`
	Hash           string          `json:"hash"`
	CreatedAt      time.Time       `json:"created_at"`
	AcknowledgedAt *time.Time      `json:"acknowledged_at,omitempty"`
	RetainUntil    time.Time       `json:"retain_until"`
}

// Journal is an append-only SQLite WAL suitable for an SD-card-backed edge agent.
// A single connection is intentional: sequence allocation and the hash-chain head
// are serialized in the same database transaction.
type Journal struct {
	mu sync.Mutex
	db *sql.DB
}

type StorageStatus struct {
	UsedBytes  int64  `json:"used_bytes"`
	QuotaBytes int64  `json:"quota_bytes"`
	Percent    int    `json:"percent"`
	State      string `json:"state"`
}

func ClassifyStorage(used, quota int64) StorageStatus {
	status := StorageStatus{UsedBytes: used, QuotaBytes: quota, State: "NORMAL"}
	if quota <= 0 {
		status.State = "UNBOUNDED"
		return status
	}
	status.Percent = int((used * 100) / quota)
	if status.Percent >= 100 {
		status.State = "FULL"
	} else if status.Percent >= 95 {
		status.State = "CRITICAL"
	} else if status.Percent >= 85 {
		status.State = "HIGH"
	} else if status.Percent >= 70 {
		status.State = "WARNING"
	}
	return status
}

// Storage reports allocated SQLite pages against the configured durable-media
// quota. It deliberately does not subtract the WAL: a conservative estimate is
// required for the fail-closed 95% command gate.
func (j *Journal) Storage(quotaBytes int64) (StorageStatus, error) {
	var pageCount, pageSize int64
	if err := j.db.QueryRow(`pragma page_count`).Scan(&pageCount); err != nil {
		return StorageStatus{}, err
	}
	if err := j.db.QueryRow(`pragma page_size`).Scan(&pageSize); err != nil {
		return StorageStatus{}, err
	}
	return ClassifyStorage(pageCount*pageSize, quotaBytes), nil
}

func Open(path string) (*Journal, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{
		`pragma journal_mode=WAL`, `pragma synchronous=FULL`, `pragma foreign_keys=ON`,
		`create table if not exists journal_events(
			sequence integer primary key,
			event_id text not null unique,
			operation_id text not null,
			event_type text not null,
			payload blob not null,
			previous_hash text not null,
			event_hash text not null unique,
			created_at text not null,
			acknowledged_at text,
			retain_until text not null
		)`,
		`create index if not exists journal_unacked on journal_events(sequence) where acknowledged_at is null`,
		`create table if not exists journal_meta(key text primary key, value text not null)`,
		`create table if not exists ble_revocations(session_id text primary key, revoked_at text not null, expires_at text not null)`,
		`create index if not exists ble_revocations_expiry on ble_revocations(expires_at)`,
		`create table if not exists sync_state(edge_id text primary key, committed_through_seq integer not null, committed_event_hash text not null, committed_at text not null)`,
		`create table if not exists sync_pending(edge_id text primary key, first_seq integer not null, last_seq integer not null, batch_sha256 text not null, check(first_seq>0 and last_seq>=first_seq))`,
		`insert or ignore into journal_meta(key,value) values('chain_anchor','')`,
		`insert or ignore into journal_meta(key,value) values('last_sequence','0')`,
	} {
		if _, err = db.Exec(q); err != nil {
			db.Close()
			return nil, err
		}
	}
	j := &Journal{db: db}
	if !j.Verify() {
		db.Close()
		return nil, errors.New("invalid hash chain")
	}
	return j, nil
}

func (j *Journal) Close() error { return j.db.Close() }

func (j *Journal) RevokeBLESession(sessionID string, revokedAt, expiresAt time.Time) error {
	if sessionID == "" || !expiresAt.After(revokedAt) {
		return errors.New("invalid BLE revocation")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	_, err := j.db.Exec(`insert into ble_revocations(session_id,revoked_at,expires_at) values(?,?,?) on conflict(session_id) do update set revoked_at=excluded.revoked_at,expires_at=excluded.expires_at`, sessionID, formatTime(revokedAt), formatTime(expiresAt))
	return err
}

func (j *Journal) IsBLESessionRevoked(sessionID string, now time.Time) bool {
	var expires string
	if j.db.QueryRow(`select expires_at from ble_revocations where session_id=?`, sessionID).Scan(&expires) != nil {
		return false
	}
	v, err := time.Parse(time.RFC3339Nano, expires)
	return err == nil && now.Before(v)
}

func (j *Journal) PurgeExpiredBLERevocations(now time.Time) (int64, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	r, err := j.db.Exec(`delete from ble_revocations where expires_at<=?`, formatTime(now))
	if err != nil {
		return 0, err
	}
	return r.RowsAffected()
}

func (j *Journal) Append(operation, typ string, payload any) (Event, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	b, err := json.Marshal(payload)
	if err != nil {
		return Event{}, err
	}
	tx, err := j.db.Begin()
	if err != nil {
		return Event{}, err
	}
	defer tx.Rollback()
	var seq int64
	var prev string
	if err = tx.QueryRow(`select cast(value as integer)+1 from journal_meta where key='last_sequence'`).Scan(&seq); err != nil {
		return Event{}, err
	}
	if tx.QueryRow(`select event_hash from journal_events order by sequence desc limit 1`).Scan(&prev) != nil {
		if err = tx.QueryRow(`select value from journal_meta where key='chain_anchor'`).Scan(&prev); err != nil {
			return Event{}, errors.New("missing chain anchor")
		}
	}
	now := time.Now().UTC()
	v := Event{Sequence: seq, EventID: fmt.Sprintf("%s-%s-%d", operation, typ, seq), OperationID: operation, Type: typ, Payload: b, PreviousHash: prev, CreatedAt: now, RetainUntil: now.AddDate(0, 3, 0)}
	v.Hash = hash(v)
	_, err = tx.Exec(`insert into journal_events(sequence,event_id,operation_id,event_type,payload,previous_hash,event_hash,created_at,retain_until) values(?,?,?,?,?,?,?,?,?)`, v.Sequence, v.EventID, v.OperationID, v.Type, []byte(v.Payload), v.PreviousHash, v.Hash, formatTime(v.CreatedAt), formatTime(v.RetainUntil))
	if err != nil {
		return Event{}, err
	}
	if _, err = tx.Exec(`update journal_meta set value=? where key='last_sequence'`, seq); err != nil {
		return Event{}, err
	}
	return v, tx.Commit()
}

func (j *Journal) Acknowledge(through int64, at time.Time) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	_, err := j.db.Exec(`update journal_events set acknowledged_at=? where sequence<=? and acknowledged_at is null`, formatTime(at), through)
	return err
}

func (j *Journal) ApplySyncAcknowledgement(edgeID string, through int64, eventHash string, at time.Time) error {
	if edgeID == "" || through < 1 || eventHash == "" {
		return errors.New("invalid sync acknowledgement")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	tx, err := j.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var previous int64
	err = tx.QueryRow(`select committed_through_seq from sync_state where edge_id=?`, edgeID).Scan(&previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if through <= previous {
		return errors.New("sync acknowledgement rollback")
	}
	if _, err = tx.Exec(`update journal_events set acknowledged_at=? where sequence<=? and acknowledged_at is null`, formatTime(at), through); err != nil {
		return err
	}
	if _, err = tx.Exec(`insert into sync_state(edge_id,committed_through_seq,committed_event_hash,committed_at) values(?,?,?,?) on conflict(edge_id) do update set committed_through_seq=excluded.committed_through_seq,committed_event_hash=excluded.committed_event_hash,committed_at=excluded.committed_at`, edgeID, through, eventHash, formatTime(at)); err != nil {
		return err
	}
	if _, err = tx.Exec(`delete from sync_pending where edge_id=?`, edgeID); err != nil {
		return err
	}
	return tx.Commit()
}

func (j *Journal) SyncState(edgeID string) (int64, string, bool) {
	var through int64
	var hash string
	err := j.db.QueryRow(`select committed_through_seq,committed_event_hash from sync_state where edge_id=?`, edgeID).Scan(&through, &hash)
	return through, hash, err == nil
}

func (j *Journal) SyncPending(edgeID string) (int64, int64, string, bool) {
	var first, last int64
	var hash string
	err := j.db.QueryRow(`select first_seq,last_seq,batch_sha256 from sync_pending where edge_id=?`, edgeID).Scan(&first, &last, &hash)
	return first, last, hash, err == nil
}

// SetSyncPending freezes the exact first transmitted batch until a valid ACK.
// Repeating the same marker is idempotent; changing it is fail-closed.
func (j *Journal) SetSyncPending(edgeID string, first, last int64, batchHash string) error {
	if edgeID == "" || first < 1 || last < first || batchHash == "" {
		return errors.New("invalid pending sync batch")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	var existingFirst, existingLast int64
	var existingHash string
	err := j.db.QueryRow(`select first_seq,last_seq,batch_sha256 from sync_pending where edge_id=?`, edgeID).Scan(&existingFirst, &existingLast, &existingHash)
	if err == nil {
		if existingFirst != first || existingLast != last || existingHash != batchHash {
			return errors.New("pending sync batch changed")
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	_, err = j.db.Exec(`insert into sync_pending(edge_id,first_seq,last_seq,batch_sha256) values(?,?,?,?)`, edgeID, first, last, batchHash)
	return err
}

func (j *Journal) Eligible(now time.Time) []Event {
	rows, err := j.db.Query(`select sequence,event_id,operation_id,event_type,payload,previous_hash,event_hash,created_at,acknowledged_at,retain_until from journal_events where acknowledged_at is not null and retain_until<=? order by sequence`, formatTime(now))
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanEvents(rows)
}

// Purge removes only backend-acknowledged records after the mandatory retention period.
func (j *Journal) Purge(now time.Time) (int64, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	tx, err := j.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	// Only a contiguous acknowledged prefix may be removed. Keeping its last
	// hash as an anchor preserves verifiability of the retained suffix.
	var through int64
	var anchor string
	rows, err := tx.Query(`select sequence,event_hash,acknowledged_at,retain_until from journal_events order by sequence`)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var seq int64
		var h string
		var ack sql.NullString
		var retain string
		if err = rows.Scan(&seq, &h, &ack, &retain); err != nil {
			rows.Close()
			return 0, err
		}
		t, e := time.Parse(time.RFC3339Nano, retain)
		if e != nil || !ack.Valid || now.Before(t) {
			break
		}
		through, anchor = seq, h
	}
	rows.Close()
	if through == 0 {
		return 0, tx.Commit()
	}
	if _, err = tx.Exec(`update journal_meta set value=? where key='chain_anchor'`, anchor); err != nil {
		return 0, err
	}
	r, err := tx.Exec(`delete from journal_events where sequence<=?`, through)
	if err != nil {
		return 0, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

func (j *Journal) Verify() bool {
	events := j.Events()
	prev := ""
	_ = j.db.QueryRow(`select value from journal_meta where key='chain_anchor'`).Scan(&prev)
	for _, v := range events {
		if v.PreviousHash != prev || hash(v) != v.Hash {
			return false
		}
		prev = v.Hash
	}
	return true
}

func (j *Journal) Events() []Event {
	rows, err := j.db.Query(`select sequence,event_id,operation_id,event_type,payload,previous_hash,event_hash,created_at,acknowledged_at,retain_until from journal_events order by sequence`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanEvents(rows)
}

func (j *Journal) Unacknowledged(limit int) []Event {
	if limit < 1 || limit > 100 {
		return nil
	}
	rows, err := j.db.Query(`select sequence,event_id,operation_id,event_type,payload,previous_hash,event_hash,created_at,acknowledged_at,retain_until from journal_events where acknowledged_at is null order by sequence limit ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	return scanEvents(rows)
}

type scanner interface{ Scan(...any) error }

func scanEvent(s scanner) (Event, error) {
	var v Event
	var created, ack, retain sql.NullString
	err := s.Scan(&v.Sequence, &v.EventID, &v.OperationID, &v.Type, &v.Payload, &v.PreviousHash, &v.Hash, &created, &ack, &retain)
	if err != nil {
		return v, err
	}
	v.CreatedAt, err = time.Parse(time.RFC3339Nano, created.String)
	if err != nil {
		return v, err
	}
	v.RetainUntil, err = time.Parse(time.RFC3339Nano, retain.String)
	if err != nil {
		return v, err
	}
	if ack.Valid {
		t, e := time.Parse(time.RFC3339Nano, ack.String)
		if e != nil {
			return v, e
		}
		v.AcknowledgedAt = &t
	}
	return v, nil
}
func scanEvents(rows *sql.Rows) []Event {
	var out []Event
	for rows.Next() {
		v, err := scanEvent(rows)
		if err != nil {
			return nil
		}
		out = append(out, v)
	}
	return out
}
func formatTime(v time.Time) string { return v.UTC().Format(time.RFC3339Nano) }
func hash(v Event) string {
	x := struct {
		Sequence                   int64
		EventID, OperationID, Type string
		Payload                    json.RawMessage
		PreviousHash               string
		CreatedAt, RetainUntil     time.Time
	}{v.Sequence, v.EventID, v.OperationID, v.Type, v.Payload, v.PreviousHash, v.CreatedAt, v.RetainUntil}
	b, _ := json.Marshal(x)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
