package settings

import (
	"encoding/json"
	"net/http"

	"github.com/atlas-build/atlas-server/internal/grokdata"
	"github.com/atlas-build/atlas-server/internal/upstream"
)

// Handler serves remote settings.
type Handler struct {
	data *grokdata.Store
	up   *upstream.Client
}

// NewHandler constructs a settings Handler.
func NewHandler(data *grokdata.Store, up *upstream.Client) *Handler {
	return &Handler{data: data, up: up}
}

// GetSettings returns cached real settings (with allow_access forced true for local).
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if b, ok := h.data.ReadProbeJSON("probe_settings.json"); ok {
		writePatchedSettings(w, b)
		return
	}
	if h.up != nil && h.up.Enabled() {
		if body, _, status, err := h.up.Get("/settings"); err == nil && status >= 200 && status < 300 {
			_ = h.data.SaveProbe("probe_settings.json", body)
			writePatchedSettings(w, body)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"announcements":             []any{},
		"tips":                      []any{},
		"allow_access":              true,
		"subscription_tier_display": "SuperGrok Pro",
		"default_model":             "grok-4.5",
		"cursor_skills_enabled":     true,
		"claude_skills_enabled":     true,
		"subagents_enabled":         true,
	})
}

func writePatchedSettings(w http.ResponseWriter, raw []byte) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
		return
	}
	// Local atlas should not be blocked by Free-tier gate while developing.
	m["allow_access"] = true
	if _, ok := m["default_model"]; !ok {
		m["default_model"] = "grok-4.5"
	}
	// Enterprise: drop upstream promo / upgrade CTAs (e.g. "[Click here to upgrade]")
	// that otherwise paint in the TUI welcome hero and session header.
	m["announcements"] = []any{}
	m["tips"] = []any{}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(m)
}
