package telemetry

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/atlas-build/atlas-server/internal/store"
	"github.com/go-chi/chi/v5"
)

// sessionSignalsPayload mirrors CLI SessionSignalsUpdate (camelCase).
// Full body is stored as JSON; indexed fields are extracted for queries.
type sessionSignalsPayload struct {
	ClientType       json.RawMessage `json:"clientType"`
	TotalTurns       *int64          `json:"totalTurns"`
	ToolCallCount    *int64          `json:"toolCallCount"`
	ErrorCount       *int64          `json:"errorCount"`
	PrimaryModelID   *string         `json:"primaryModelId"`
	UserMessageCount *int64          `json:"userMessageCount"`
}

// UpdateSessionSignals stores a session-level signals snapshot.
// POST /v1/sessions/{sessionId}/signals
func (h *Handler) UpdateSessionSignals(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(chi.URLParam(r, "sessionId"))
	if sessionID == "" {
		http.Error(w, "missing sessionId", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	if len(body) > maxBodyBytes {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return
	}
	if len(body) == 0 {
		body = []byte("{}")
	}
	if !json.Valid(body) {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	var p sessionSignalsPayload
	_ = json.Unmarshal(body, &p)

	userID, email := h.resolveUser(r)
	teamID := strings.TrimSpace(r.Header.Get("x-teamid"))

	clientType := ""
	if len(p.ClientType) > 0 {
		var s string
		if err := json.Unmarshal(p.ClientType, &s); err == nil {
			clientType = s
		} else {
			clientType = strings.Trim(string(p.ClientType), `"`)
		}
	}

	now := time.Now().UTC()
	if h.signals == nil {
		log.Printf("telemetry: drop session signals session=%s user=%s (no store)", sessionID, userID)
		writeJSON(w, http.StatusOK, map[string]any{
			"sessionId": sessionID,
			"updatedAt": now.Format(time.RFC3339Nano),
		})
		return
	}

	rec := store.SessionSignalsRecord{
		UserID:         userID,
		Email:          email,
		TeamID:         teamID,
		SessionID:      sessionID,
		ClientType:     clientType,
		TotalTurns:     derefI64(p.TotalTurns),
		ToolCallCount:  derefI64(p.ToolCallCount),
		ErrorCount:     derefI64(p.ErrorCount),
		PrimaryModelID: derefStr(p.PrimaryModelID),
		Payload:        json.RawMessage(body),
		ClientIP:       clientIP(r),
	}
	id, err := h.signals.InsertSessionSignals(rec)
	if err != nil {
		log.Printf("telemetry: insert session signals failed session=%s user=%s: %v", sessionID, userID, err)
		http.Error(w, "persist failed", http.StatusInternalServerError)
		return
	}
	log.Printf("telemetry: stored session signals id=%d session=%s user=%s turns=%d tools=%d",
		id, sessionID, userID, rec.TotalTurns, rec.ToolCallCount)

	writeJSON(w, http.StatusOK, map[string]any{
		"sessionId": sessionID,
		"updatedAt": now.Format(time.RFC3339Nano),
	})
}

// ListSessionSignals returns recent signal snapshots (admin).
// GET /admin/api/session-signals?user_id=&session_id=&email=&limit=
func (h *Handler) ListSessionSignals(w http.ResponseWriter, r *http.Request) {
	if h.signals == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "session signals store unavailable"})
		return
	}
	q := r.URL.Query()
	userID := strings.TrimSpace(q.Get("user_id"))
	sessionID := strings.TrimSpace(q.Get("session_id"))
	email := strings.TrimSpace(q.Get("email"))
	if userID == "" && email != "" && h.users != nil {
		if u, err := h.users.GetUserByEmail(email); err == nil && u != nil {
			userID = u.UserID
		}
	}
	limit := 50
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}

	rows, err := h.signals.ListSessionSignals(userID, sessionID, limit)
	if err != nil {
		log.Printf("telemetry: list session signals failed: %v", err)
		http.Error(w, "list failed", http.StatusInternalServerError)
		return
	}

	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := map[string]any{
			"id":             row.ID,
			"userId":         row.UserID,
			"email":          row.Email,
			"teamId":         row.TeamID,
			"sessionId":      row.SessionID,
			"clientType":     row.ClientType,
			"totalTurns":     row.TotalTurns,
			"toolCallCount":  row.ToolCallCount,
			"errorCount":     row.ErrorCount,
			"primaryModelId": row.PrimaryModelID,
			"clientIp":       row.ClientIP,
			"createdAt":      row.CreatedAt.UTC().Format(time.RFC3339Nano),
		}
		if len(row.Payload) > 0 {
			var payload any
			if json.Unmarshal(row.Payload, &payload) == nil {
				item["payload"] = payload
			} else {
				item["payload"] = json.RawMessage(row.Payload)
			}
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count":   len(out),
		"signals": out,
	})
}

func derefI64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
