package user

import (
	"encoding/json"
	"net/http"

	"github.com/atlas-build/atlas-server/internal/auth"
	"github.com/atlas-build/atlas-server/internal/config"
	"github.com/atlas-build/atlas-server/internal/grokdata"
	"github.com/atlas-build/atlas-server/internal/store"
	"github.com/atlas-build/atlas-server/internal/upstream"
)

// Handler serves GET /v1/user.
type Handler struct {
	cfg   config.Config
	auth  *auth.Handler
	users store.UserStore
	data  *grokdata.Store
	up    *upstream.Client
}

// NewHandler constructs a user Handler.
func NewHandler(cfg config.Config, authH *auth.Handler, users store.UserStore, data *grokdata.Store, up *upstream.Client) *Handler {
	return &Handler{cfg: cfg, auth: authH, users: users, data: data, up: up}
}

// GetUser prefers cached real user payload, else builds a local one.
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID, email, ok := h.auth.ParseBearerUser(r)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	if b, ok := h.data.ReadProbeJSON("probe_user_include=subscription.json"); ok {
		var m map[string]any
		if json.Unmarshal(b, &m) == nil {
			// Keep local login identity while preserving real subscription fields.
			if userID != "" {
				m["userId"] = userID
				m["principalId"] = userID
			}
			if email != "" {
				m["email"] = email
			}
			// Ensure subscriptionTier qualifies local paywall when real is Free/null.
			if m["subscriptionTier"] == nil || m["subscriptionTier"] == "" {
				m["subscriptionTier"] = "SuperGrokPro"
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(m)
			return
		}
	}

	if h.up != nil && h.up.Enabled() {
		if body, _, status, err := h.up.Get("/user?include=subscription"); err == nil && status >= 200 && status < 300 {
			_ = h.data.SaveProbe("probe_user_include=subscription.json", body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
			return
		}
	}

	if h.users != nil {
		if u, err := h.users.GetUserByID(userID); err == nil && u != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(u.ToProfile())
			return
		}
	}

	if email == "" {
		email = h.cfg.DefaultEmail
	}
	if userID == "" {
		userID = h.cfg.DefaultUserID
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"userId":                    userID,
		"email":                     email,
		"firstName":                 "Atlas",
		"lastName":                  "Dev",
		"principalType":             "User",
		"principalId":               userID,
		"codingDataRetentionOptOut": false,
		"subscriptionTier":          "SuperGrokPro",
	})
}
