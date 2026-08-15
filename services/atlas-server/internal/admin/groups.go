package admin

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/atlas-build/atlas-server/internal/store"
	"github.com/go-chi/chi/v5"
)

func groupJSON(g *store.UserGroup) map[string]any {
	return map[string]any{
		"group_id":   g.GroupID,
		"name":       g.Name,
		"created_at": g.CreatedAt,
		"updated_at": g.UpdatedAt,
	}
}

// ListUserGroups GET /admin/api/groups
func (h *ModelsHandler) ListUserGroups(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		writeJSON(w, http.StatusOK, map[string]any{"groups": []any{}})
		return
	}
	list, err := h.models.ListUserGroups()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []store.UserGroupBrief{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"groups": list})
}

// CreateUserGroup POST /admin/api/groups  { "name": "..." }
func (h *ModelsHandler) CreateUserGroup(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	g, err := h.models.CreateUserGroup(req.Name)
	if err != nil {
		if errors.Is(err, store.ErrUserGroupNameTaken) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "group": groupJSON(g)})
}

// UpdateUserGroup PUT /admin/api/groups/{groupId}  { "name": "..." }
func (h *ModelsHandler) UpdateUserGroup(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		return
	}
	groupID := chi.URLParam(r, "groupId")
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	g, err := h.models.UpdateUserGroup(groupID, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrUserGroupNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, store.ErrUserGroupNameTaken) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "group": groupJSON(g)})
}

// DeleteUserGroup DELETE /admin/api/groups/{groupId}
func (h *ModelsHandler) DeleteUserGroup(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		return
	}
	if err := h.models.DeleteUserGroup(chi.URLParam(r, "groupId")); err != nil {
		if errors.Is(err, store.ErrUserGroupNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// GetGroupMembers GET /admin/api/groups/{groupId}/members
func (h *ModelsHandler) GetGroupMembers(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		writeJSON(w, http.StatusOK, map[string]any{"user_ids": []any{}})
		return
	}
	groupID := chi.URLParam(r, "groupId")
	ids, err := h.models.ListGroupMemberIDs(groupID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ids == nil {
		ids = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"group_id": groupID, "user_ids": ids})
}

// SetGroupMembers PUT /admin/api/groups/{groupId}/members  { "user_ids": [...] }
func (h *ModelsHandler) SetGroupMembers(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		return
	}
	groupID := chi.URLParam(r, "groupId")
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var req struct {
		UserIDs []string `json:"user_ids"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := h.models.SetGroupMembers(groupID, req.UserIDs); err != nil {
		if errors.Is(err, store.ErrUserGroupNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// GetGroupModels GET /admin/api/groups/{groupId}/models
func (h *ModelsHandler) GetGroupModels(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		writeJSON(w, http.StatusOK, map[string]any{"model_ids": []any{}})
		return
	}
	groupID := chi.URLParam(r, "groupId")
	ids, err := h.models.ListGroupModelIDs(groupID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if ids == nil {
		ids = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"group_id": groupID, "model_ids": ids})
}

// SetGroupModels PUT /admin/api/groups/{groupId}/models  { "model_ids": [...] }
func (h *ModelsHandler) SetGroupModels(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		http.Error(w, "store unavailable", http.StatusServiceUnavailable)
		return
	}
	groupID := chi.URLParam(r, "groupId")
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	var req struct {
		ModelIDs []string `json:"model_ids"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := h.models.SetGroupModels(groupID, req.ModelIDs); err != nil {
		if errors.Is(err, store.ErrUserGroupNotFound) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// GetEffectiveModels GET /admin/api/users/{userId}/effective-models
func (h *ModelsHandler) GetEffectiveModels(w http.ResponseWriter, r *http.Request) {
	if h.models == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"user_id": strings.TrimSpace(chi.URLParam(r, "userId")),
			"model_ids": []any{}, "default_model": "", "sources": []any{},
		})
		return
	}
	userID := chi.URLParam(r, "userId")
	eff, err := h.models.ListEffectiveModels(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, eff)
}
