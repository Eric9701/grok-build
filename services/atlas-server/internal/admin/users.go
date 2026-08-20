package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/atlas-build/atlas-server/internal/store"
)

const defaultPassword = "atlas123"

// UsersHandler serves user management admin APIs.
type UsersHandler struct {
	users store.UserStore
}

// NewUsersHandler constructs a UsersHandler.
func NewUsersHandler(users store.UserStore) *UsersHandler {
	return &UsersHandler{users: users}
}

type createUserRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	MachineCode string `json:"machineCode"`
}


// CreateUser creates a new user with email prefix as userId and default password.
// POST /admin/api/users
func (h *UsersHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	var req createUserRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	req.Name = strings.TrimSpace(req.Name)
	req.MachineCode = strings.TrimSpace(req.MachineCode)

	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}

	// userId = email prefix (before @)
	userID := req.Email
	if idx := strings.Index(req.Email, "@"); idx > 0 {
		userID = req.Email[:idx]
	}

	// Split name into first/last
	firstName, lastName := splitName(req.Name)

	// Generate machine code if not provided
	machineCode := req.MachineCode
	if machineCode == "" {
		mc, err := store.GenerateMachineCode()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "generate machine code failed"})
			return
		}
		machineCode = mc
	}

	hash, err := store.HashPassword(defaultPassword)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "hash password failed"})
		return
	}

	u := store.User{
		UserID:        userID,
		Email:         req.Email,
		PasswordHash:  hash,
		FirstName:     firstName,
		LastName:      lastName,
		PrincipalType: "User",
		PrincipalID:   userID,
		MachineCode:   machineCode,
	}
	if err := h.users.CreateUser(u); err != nil {
		if err == store.ErrUserExists {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "user already exists"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"userId":      u.UserID,
		"email":       u.Email,
		"firstName":   u.FirstName,
		"lastName":    u.LastName,
		"machineCode": u.MachineCode,
	})
}

// ListAllUsers returns all users for admin management.
// GET /admin/api/users/all
func (h *UsersHandler) ListAllUsers(w http.ResponseWriter, r *http.Request) {
	type detailStore interface {
		ListUsersDetail() ([]store.UserDetail, error)
	}
	ls, ok := h.users.(detailStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "user list unavailable"})
		return
	}
	users, err := ls.ListUsersDetail()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func splitName(name string) (first, last string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Atlas", "User"
	}
	parts := strings.Fields(name)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}
