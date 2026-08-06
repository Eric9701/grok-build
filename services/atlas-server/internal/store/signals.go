package store

import (
	"encoding/json"
	"time"
)

// SessionSignalsRecord is one CLI session-signals snapshot posted to
// POST /v1/sessions/{sessionId}/signals.
type SessionSignalsRecord struct {
	ID             int64
	UserID         string
	Email          string
	TeamID         string
	SessionID      string
	ClientType     string
	TotalTurns     int64
	ToolCallCount  int64
	ErrorCount     int64
	PrimaryModelID string
	Payload        json.RawMessage
	ClientIP       string
	CreatedAt      time.Time
}

// SessionSignalsStore persists session signal snapshots.
type SessionSignalsStore interface {
	InsertSessionSignals(r SessionSignalsRecord) (int64, error)
	ListSessionSignals(userID, sessionID string, limit int) ([]SessionSignalsRecord, error)
}

// InsertSessionSignals stores one cumulative signals snapshot.
func (m *MySQLStore) InsertSessionSignals(r SessionSignalsRecord) (int64, error) {
	if len(r.Payload) == 0 {
		r.Payload = json.RawMessage(`{}`)
	}
	res, err := m.db.Exec(
		`INSERT INTO session_signals
			(user_id, email, team_id, session_id, client_type,
			 total_turns, tool_call_count, error_count, primary_model_id,
			 payload, client_ip)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullIfEmpty(r.UserID),
		nullIfEmpty(r.Email),
		nullIfEmpty(r.TeamID),
		r.SessionID,
		nullIfEmpty(r.ClientType),
		r.TotalTurns,
		r.ToolCallCount,
		r.ErrorCount,
		nullIfEmpty(r.PrimaryModelID),
		r.Payload,
		nullIfEmpty(r.ClientIP),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListSessionSignals returns recent snapshots (newest first). Payload included.
func (m *MySQLStore) ListSessionSignals(userID, sessionID string, limit int) ([]SessionSignalsRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	query := `SELECT id, IFNULL(user_id,''), IFNULL(email,''), IFNULL(team_id,''),
		session_id, IFNULL(client_type,''), total_turns, tool_call_count, error_count,
		IFNULL(primary_model_id,''), payload, IFNULL(client_ip,''), created_at
		FROM session_signals WHERE 1=1`
	args := make([]any, 0, 3)
	if userID != "" {
		query += ` AND user_id = ?`
		args = append(args, userID)
	}
	if sessionID != "" {
		query += ` AND session_id = ?`
		args = append(args, sessionID)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionSignalsRecord
	for rows.Next() {
		var r SessionSignalsRecord
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.Email, &r.TeamID,
			&r.SessionID, &r.ClientType, &r.TotalTurns, &r.ToolCallCount, &r.ErrorCount,
			&r.PrimaryModelID, &r.Payload, &r.ClientIP, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

var _ SessionSignalsStore = (*MySQLStore)(nil)
