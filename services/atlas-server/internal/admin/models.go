package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/atlas-build/atlas-server/internal/auth"
	"github.com/atlas-build/atlas-server/internal/crypto"
	"github.com/atlas-build/atlas-server/internal/store"
	"github.com/go-chi/chi/v5"
)

const maxBody = 1 << 20

// ModelsHandler serves managed-model admin + crypto APIs.
type ModelsHandler struct {
	auth   *auth.Handler
	models store.ManagedModelStore
	secret string
}

// NewModelsHandler constructs a ModelsHandler.
func NewModelsHandler(authH *auth.Handler, models store.ManagedModelStore, secret string) *ModelsHandler {
	if secret == "" {
		secret = crypto.ResolveModelSecret()
	}
	return &ModelsHandler{auth: authH, models: models, secret: secret}
}

type managedModelJSON struct {
	ID            string `json:"id"`
	Model         string `json:"model"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	BaseURL       string `json:"base_url"`
	APIBackend    string `json:"api_backend"`
	APIKey        string `json:"api_key,omitempty"`      // write: plaintext or ENC
	APIKeyMasked  string `json:"api_key_masked,omitempty"` // read
	HasKey        bool   `json:"has_key"`
	ContextWindow int64  `json:"context_window"`
	OwnedBy       string `json:"owned_by,omitempty"`
	Enabled       bool   `json:"enabled"`
	IsDefault     bool   `json:"is_default,omitempty"`
}

func toJSON(m store.ManagedModel, mask bool) managedModelJSON {
	out := managedModelJSON{
		ID:            m.ID,
		Model:         m.Model,
		Name:          m.Name,
		Description:   m.Description,
		BaseURL:       m.BaseURL,
		APIBackend:    m.APIBackend,
		ContextWindow: m.ContextWindow,
		OwnedBy:       m.OwnedBy,
		Enabled:       m.Enabled,
		HasKey:        strings.TrimSpace(m.APIKeyEnc) != "",
	}
	if mask {
		out.APIKeyMasked = crypto.MaskEnc(m.APIKeyEnc)
	} else {
		out.APIKey = m.APIKeyEnc
	}
	return out
}

// ListManagedModels GET /admin/api/managed-models
func (h *ModelsHandler) ListManagedModels(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		writeJSON(w, http.StatusOK, map[string]any{"models": []any{}})
		return
	}
	list, err := h.models.ListManagedModels()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := make([]managedModelJSON, 0, len(list))
	for _, m := range list {
		out = append(out, toJSON(m, true))
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": out})
}

// UpsertManagedModel POST/PUT /admin/api/managed-models[/{id}]
func (h *ModelsHandler) UpsertManagedModel(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var req managedModelJSON
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if id := chi.URLParam(r, "id"); id != "" {
		req.ID = id
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Model = strings.TrimSpace(req.Model)
	if req.ID == "" || req.Model == "" {
		http.Error(w, "id and model required", http.StatusBadRequest)
		return
	}
	mm := store.ManagedModel{
		ID:            req.ID,
		Model:         req.Model,
		Name:          strings.TrimSpace(req.Name),
		Description:   req.Description,
		BaseURL:       strings.TrimSpace(req.BaseURL),
		APIBackend:    strings.TrimSpace(req.APIBackend),
		ContextWindow: req.ContextWindow,
		OwnedBy:       req.OwnedBy,
		Enabled:       req.Enabled,
	}
	if r.Method == http.MethodPost {
		mm.Enabled = true
		if !req.Enabled && strings.Contains(string(body), `"enabled"`) {
			mm.Enabled = req.Enabled
		}
		if !strings.Contains(string(body), `"enabled"`) {
			mm.Enabled = true
		}
	}
	if key := strings.TrimSpace(req.APIKey); key != "" {
		enc, err := crypto.Encrypt(key, h.secret)
		if err != nil {
			http.Error(w, "encrypt failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		mm.APIKeyEnc = enc
	} else if existing, err := h.models.GetManagedModel(req.ID); err == nil && existing != nil {
		mm.APIKeyEnc = existing.APIKeyEnc
		if r.Method == http.MethodPost {
			// keep
		}
	}
	if mm.APIKeyEnc == "" && r.Method == http.MethodPost {
		http.Error(w, "api_key required on create", http.StatusBadRequest)
		return
	}
	if err := h.models.UpsertManagedModel(mm); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	saved, _ := h.models.GetManagedModel(req.ID)
	if saved == nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": req.ID})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "model": toJSON(*saved, true)})
}

// DeleteManagedModel DELETE /admin/api/managed-models/{id}
func (h *ModelsHandler) DeleteManagedModel(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.models.DeleteManagedModel(id); err != nil {
		if err == store.ErrManagedModelNotFound {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// ListUsers GET /admin/api/users
func (h *ModelsHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		writeJSON(w, http.StatusOK, map[string]any{"users": []any{}})
		return
	}
	users, err := h.models.ListUsersBrief()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

// GetUserModels GET /admin/api/users/{userId}/models
func (h *ModelsHandler) GetUserModels(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		writeJSON(w, http.StatusOK, map[string]any{"model_ids": []any{}})
		return
	}
	userID := chi.URLParam(r, "userId")
	as, err := h.models.ListUserModelIDs(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ids := make([]string, 0, len(as))
	def := ""
	for _, a := range as {
		ids = append(ids, a.ModelID)
		if a.IsDefault {
			def = a.ModelID
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_id": userID, "model_ids": ids, "default_model": def})
}

// SetUserModels PUT /admin/api/users/{userId}/models
func (h *ModelsHandler) SetUserModels(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		return
	}
	userID := chi.URLParam(r, "userId")
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var req struct {
		ModelIDs      []string `json:"model_ids"`
		DefaultModel  string   `json:"default_model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := h.models.SetUserModels(userID, req.ModelIDs, req.DefaultModel); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// Encrypt POST /admin/api/crypto/encrypt
func (h *ModelsHandler) Encrypt(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var req struct {
		Plaintext string `json:"plaintext"`
	}
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.Plaintext) == "" {
		http.Error(w, "plaintext required", http.StatusBadRequest)
		return
	}
	enc, err := crypto.Encrypt(req.Plaintext, h.secret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enc": enc})
}

// Decrypt POST /admin/api/crypto/decrypt
func (h *ModelsHandler) Decrypt(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var req struct {
		Enc string `json:"enc"`
	}
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.Enc) == "" {
		http.Error(w, "enc required", http.StatusBadRequest)
		return
	}
	plain, err := crypto.Decrypt(req.Enc, h.secret)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plaintext": plain})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
