package telemetry

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/atlas-build/atlas-server/internal/auth"
	"github.com/atlas-build/atlas-server/internal/store"
)

const maxBodyBytes = 16 << 20 // 16 MiB

// Handler ingests CLI telemetry and persists traces / task reports / session signals by user.
type Handler struct {
	auth    *auth.Handler
	traces  store.TraceStore
	reports store.TaskReportStore
	signals store.SessionSignalsStore
	users   store.UserStore
}

// NewHandler constructs a telemetry Handler.
func NewHandler(
	authH *auth.Handler,
	traces store.TraceStore,
	reports store.TaskReportStore,
	signals store.SessionSignalsStore,
	users store.UserStore,
) *Handler {
	return &Handler{auth: authH, traces: traces, reports: reports, signals: signals, users: users}
}

// IngestEvents accepts product events and acknowledges (still no-op persist).
func (h *Handler) IngestEvents(w http.ResponseWriter, r *http.Request) {
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, maxBodyBytes))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// Traces stores an OTLP batch attributed to the authenticated / header user.
func (h *Handler) Traces(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	if len(body) > maxBodyBytes {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return
	}

	userID, email := h.resolveUser(r)
	teamID := strings.TrimSpace(r.Header.Get("x-teamid"))
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/x-protobuf"
	}

	if h.traces == nil {
		log.Printf("telemetry: drop trace user=%s size=%d (no store)", userID, len(body))
		w.WriteHeader(http.StatusAccepted)
		return
	}

	id, err := h.traces.InsertTrace(store.TraceRecord{
		UserID:      userID,
		Email:       email,
		TeamID:      teamID,
		ContentType: ct,
		Body:        body,
		BodySize:    len(body),
		ClientIP:    clientIP(r),
	})
	if err != nil {
		log.Printf("telemetry: insert trace failed user=%s: %v", userID, err)
		http.Error(w, "persist failed", http.StatusInternalServerError)
		return
	}
	log.Printf("telemetry: stored trace id=%d user=%s email=%s size=%d", id, userID, email, len(body))
	w.WriteHeader(http.StatusAccepted)
}

// ListTraces returns recent traces for a user (admin).
// GET /admin/api/traces?user_id=...&limit=50
func (h *Handler) ListTraces(w http.ResponseWriter, r *http.Request) {
	if h.traces == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "trace store unavailable"})
		return
	}
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		if email := strings.TrimSpace(r.URL.Query().Get("email")); email != "" && h.users != nil {
			if u, err := h.users.GetUserByEmail(email); err == nil && u != nil {
				userID = u.UserID
			}
		}
	}
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id or email required"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	rows, err := h.traces.ListTracesByUser(userID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type item struct {
		ID          int64  `json:"id"`
		UserID      string `json:"userId"`
		Email       string `json:"email"`
		TeamID      string `json:"teamId,omitempty"`
		ContentType string `json:"contentType"`
		BodySize    int    `json:"bodySize"`
		ClientIP    string `json:"clientIp,omitempty"`
		CreatedAt   string `json:"createdAt"`
	}
	out := make([]item, 0, len(rows))
	for _, row := range rows {
		out = append(out, item{
			ID:          row.ID,
			UserID:      row.UserID,
			Email:       row.Email,
			TeamID:      row.TeamID,
			ContentType: row.ContentType,
			BodySize:    row.BodySize,
			ClientIP:    row.ClientIP,
			CreatedAt:   row.CreatedAt.UTC().Format(timeRFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"userId": userID,
		"count":  len(out),
		"traces": out,
	})
}

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"

func (h *Handler) resolveUser(r *http.Request) (userID, email string) {
	if h.auth != nil {
		if uid, em, ok := h.auth.ParseBearerUser(r); ok {
			userID, email = uid, em
		}
	}
	if hdr := strings.TrimSpace(r.Header.Get("x-userid")); hdr != "" {
		if userID == "" {
			userID = hdr
		}
	}
	if email == "" && userID != "" && h.users != nil {
		if u, err := h.users.GetUserByID(userID); err == nil && u != nil {
			email = u.Email
			if userID == "" {
				userID = u.UserID
			}
		}
	}
	if userID == "" {
		userID = "anonymous"
	}
	return userID, email
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if rip := r.Header.Get("X-Real-IP"); rip != "" {
		return strings.TrimSpace(rip)
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		return host[:i]
	}
	return host
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
