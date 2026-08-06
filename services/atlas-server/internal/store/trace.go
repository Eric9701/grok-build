package store

import (
	"time"
)

// TraceRecord is one ingested OTLP (or raw) trace batch, attributed to a user.
type TraceRecord struct {
	ID          int64
	UserID      string
	Email       string
	TeamID      string
	ContentType string
	Body        []byte
	BodySize    int
	ClientIP    string
	CreatedAt   time.Time
}

// TraceStore persists CLI OTLP trace batches.
type TraceStore interface {
	InsertTrace(r TraceRecord) (int64, error)
	ListTracesByUser(userID string, limit int) ([]TraceRecord, error)
}

// InsertTrace stores a raw OTLP body keyed by user.
func (m *MySQLStore) InsertTrace(r TraceRecord) (int64, error) {
	if r.BodySize == 0 {
		r.BodySize = len(r.Body)
	}
	res, err := m.db.Exec(
		`INSERT INTO telemetry_traces
			(user_id, email, team_id, content_type, body, body_size, client_ip)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		nullIfEmpty(r.UserID),
		nullIfEmpty(r.Email),
		nullIfEmpty(r.TeamID),
		r.ContentType,
		r.Body,
		r.BodySize,
		nullIfEmpty(r.ClientIP),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListTracesByUser returns recent traces for a user (newest first). Body omitted for list.
func (m *MySQLStore) ListTracesByUser(userID string, limit int) ([]TraceRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := m.db.Query(
		`SELECT id, IFNULL(user_id,''), IFNULL(email,''), IFNULL(team_id,''),
		        content_type, body_size, IFNULL(client_ip,''), created_at
		 FROM telemetry_traces
		 WHERE user_id = ?
		 ORDER BY id DESC
		 LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TraceRecord
	for rows.Next() {
		var r TraceRecord
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.Email, &r.TeamID,
			&r.ContentType, &r.BodySize, &r.ClientIP, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

var _ TraceStore = (*MySQLStore)(nil)
