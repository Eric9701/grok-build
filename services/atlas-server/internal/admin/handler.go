package admin

import (
	"encoding/json"
	"net/http"
)

// Handler is a placeholder for the future admin / management API.
type Handler struct{}

// NewHandler constructs a placeholder admin Handler.
func NewHandler() *Handler { return &Handler{} }

// Status returns a simple service status for operators.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"service": "atlas-server",
		"modules": map[string]string{
			"auth":      "ready",
			"settings":  "stub",
			"skills":    "stub",
			"telemetry": "ready",
			"models":    "ready",
			"admin":     "ready",
		},
	})
}
