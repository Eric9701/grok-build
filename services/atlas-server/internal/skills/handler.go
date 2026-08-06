package skills

import (
	"encoding/json"
	"net/http"
)

// Handler is a placeholder for skill package distribution.
type Handler struct{}

// NewHandler constructs a placeholder skills Handler.
func NewHandler() *Handler { return &Handler{} }

// List returns 501 — skill sync will be implemented later.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "not_implemented",
		"message": "skill distribution will be added in a later release",
	})
}
