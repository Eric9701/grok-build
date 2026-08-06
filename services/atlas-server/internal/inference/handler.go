package inference

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/atlas-build/atlas-server/internal/auth"
	"github.com/atlas-build/atlas-server/internal/crypto"
	"github.com/atlas-build/atlas-server/internal/grokdata"
	"github.com/atlas-build/atlas-server/internal/store"
	"github.com/atlas-build/atlas-server/internal/upstream"
)

// Handler serves model catalog and Responses API.
type Handler struct {
	data   *grokdata.Store
	up     *upstream.Client
	auth   *auth.Handler
	models store.ManagedModelStore
	users  store.UserStore
}

// NewHandler constructs an inference Handler.
func NewHandler(data *grokdata.Store, up *upstream.Client) *Handler {
	return &Handler{data: data, up: up}
}

// WithManagedModels attaches per-user managed model catalog support.
func (h *Handler) WithManagedModels(authH *auth.Handler, models store.ManagedModelStore, users store.UserStore) *Handler {
	h.auth = authH
	h.models = models
	h.users = users
	return h
}

// ListModels prefers per-user managed models, then probe cache, upstream, builtin.
func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	if h.tryManagedModels(w, r) {
		return
	}
	if b, ok := h.data.ReadProbeJSON("probe_models.json"); ok {
		writeRawJSON(w, http.StatusOK, b)
		return
	}
	if h.up != nil && h.up.Enabled() {
		if body, _, status, err := h.up.Get("/models"); err == nil && status >= 200 && status < 300 {
			_ = h.data.SaveProbe("probe_models.json", body)
			writeRawJSON(w, http.StatusOK, body)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"object": "list",
		"data": []map[string]any{
			{
				"id":                        "grok-4.5",
				"object":                    "model",
				"owned_by":                  "xAI",
				"model":                     "grok-4.5",
				"name":                      "Grok 4.5",
				"description":               "Atlas local fallback model",
				"context_window":            500000,
				"api_backend":               "responses",
				"reasoning_effort":          "high",
				"supports_reasoning_effort": true,
			},
		},
	})
}

func (h *Handler) tryManagedModels(w http.ResponseWriter, r *http.Request) bool {
	if h.models == nil {
		return false
	}
	userID := h.resolveUserID(r)
	if userID == "" || userID == "anonymous" {
		return false
	}
	list, err := h.models.ListModelsForUser(userID)
	if err != nil || len(list) == 0 {
		return false
	}
	secret := crypto.ResolveModelSecret()
	data := make([]map[string]any, 0, len(list))
	for _, m := range list {
		encID, err := crypto.Encrypt(m.ID, secret)
		if err != nil {
			continue
		}
		encModel, err := crypto.Encrypt(m.Model, secret)
		if err != nil {
			continue
		}
		entry := map[string]any{
			"id":             encID,
			"object":         "model",
			"owned_by":       m.OwnedBy,
			"model":          encModel,
			"name":           m.Name,
			"description":    m.Description,
			"base_url":       m.BaseURL,
			"api_backend":    m.APIBackend,
			"context_window": m.ContextWindow,
			"managed":        true,
		}
		if m.APIKeyEnc != "" {
			entry["api_key"] = m.APIKeyEnc
		}
		data = append(data, entry)
	}
	if len(data) == 0 {
		return false
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"object":  "list",
		"data":    data,
		"managed": true,
	})
	return true
}

func (h *Handler) resolveUserID(r *http.Request) string {
	userID := ""
	if h.auth != nil {
		if uid, _, ok := h.auth.ParseBearerUser(r); ok {
			userID = uid
		}
	}
	if hdr := strings.TrimSpace(r.Header.Get("x-userid")); hdr != "" && userID == "" {
		userID = hdr
	}
	if userID != "" && h.users != nil {
		if u, err := h.users.GetUserByID(userID); err == nil && u != nil {
			return u.UserID
		}
	}
	return userID
}

// Responses proxies to real xAI using ~/.grok auth when possible; else SSE echo.
func (h *Handler) Responses(w http.ResponseWriter, r *http.Request) {
	if h.up != nil && h.up.Enabled() {
		if err := h.up.ProxyStream(w, r, "/responses"); err == nil {
			return
		}
		// fall through to echo on upstream failure
	}
	h.echoResponses(w, r)
}

func (h *Handler) echoResponses(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var req map[string]any
	_ = json.Unmarshal(body, &req)
	model, _ := req["model"].(string)
	if model == "" {
		model = "grok-4.5"
	}
	userMsg := extractUserText(req)
	if userMsg == "" {
		userMsg = "hello"
	}
	reply := fmt.Sprintf("Atlas local echo (upstream unavailable): %s", userMsg)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	seq := 0
	writeSSE(w, flusher, map[string]any{
		"type":            "response.created",
		"sequence_number": seq,
		"response": map[string]any{
			"id":     "resp_atlas_local",
			"object": "response",
			"model":  model,
			"status": "in_progress",
		},
	})
	seq++
	itemID := "msg_atlas_local"
	writeSSE(w, flusher, map[string]any{
		"type":            "response.output_item.added",
		"sequence_number": seq,
		"output_index":    0,
		"item": map[string]any{
			"type":   "message",
			"id":     itemID,
			"status": "in_progress",
			"role":   "assistant",
		},
	})
	seq++
	writeSSE(w, flusher, map[string]any{
		"type":            "response.output_text.delta",
		"sequence_number": seq,
		"item_id":         itemID,
		"output_index":    0,
		"content_index":   0,
		"delta":           reply,
	})
	seq++
	writeSSE(w, flusher, map[string]any{
		"type":            "response.completed",
		"sequence_number": seq,
		"response": map[string]any{
			"id":        "resp_atlas_local",
			"object":    "response",
			"model":     model,
			"status":    "completed",
			"created_at": time.Now().Unix(),
		},
	})
}

func extractUserText(req map[string]any) string {
	if input, ok := req["input"].(string); ok {
		return input
	}
	if arr, ok := req["input"].([]any); ok {
		for _, it := range arr {
			m, ok := it.(map[string]any)
			if !ok {
				continue
			}
			if role, _ := m["role"].(string); role == "user" {
				if c, ok := m["content"].(string); ok {
					return c
				}
			}
		}
	}
	return ""
}

func writeSSE(w http.ResponseWriter, f http.Flusher, v any) {
	b, _ := json.Marshal(v)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
	f.Flush()
}

func writeRawJSON(w http.ResponseWriter, status int, b []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}
