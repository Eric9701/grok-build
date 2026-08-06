package account

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/atlas-build/atlas-server/internal/config"
	"github.com/atlas-build/atlas-server/internal/store"
	"github.com/google/uuid"
)

const sessionCookie = "atlas_session"

// Handler serves account login/register and session APIs.
type Handler struct {
	cfg   config.Config
	users store.UserStore
	login *template.Template
}

// NewHandler constructs an account Handler.
func NewHandler(cfg config.Config, users store.UserStore, loginPage *template.Template) *Handler {
	return &Handler{cfg: cfg, users: users, login: loginPage}
}

// LoginPage serves GET/POST /login.
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	next := safeNext(r.FormValue("next"))
	if r.Method == http.MethodGet {
		if next == "" {
			next = r.URL.Query().Get("next")
		}
		if u, _ := h.users.GetUserBySession(h.sessionToken(r)); u != nil {
			http.Redirect(w, r, h.redirectTarget(next), http.StatusFound)
			return
		}
		h.renderLogin(w, next, "", false)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderLogin(w, next, "invalid form", false)
		return
	}
	next = safeNext(r.FormValue("next"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	u, err := h.users.GetUserByEmail(email)
	if err != nil || store.CheckPassword(u.PasswordHash, password) != nil {
		h.renderLogin(w, next, "invalid email or password", false)
		return
	}
	if err := h.setSession(w, u.UserID); err != nil {
		h.renderLogin(w, next, "failed to create session", false)
		return
	}
	http.Redirect(w, r, h.redirectTarget(next), http.StatusFound)
}

// RegisterPage serves GET/POST /register.
func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	next := safeNext(r.URL.Query().Get("next"))
	if r.Method == http.MethodGet {
		h.renderLogin(w, next, "", true)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderLogin(w, next, "invalid form", true)
		return
	}
	next = safeNext(r.FormValue("next"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	first := strings.TrimSpace(r.FormValue("first_name"))
	last := strings.TrimSpace(r.FormValue("last_name"))
	if email == "" || len(password) < 8 {
		h.renderLogin(w, next, "email required and password must be at least 8 characters", true)
		return
	}
	if first == "" {
		first = "Atlas"
	}
	if last == "" {
		last = "User"
	}
	userID := "user-" + uuid.NewString()
	machineCode, err := store.GenerateMachineCode()
	if err != nil {
		h.renderLogin(w, next, "failed to generate machine code", true)
		return
	}
	hash, err := store.HashPassword(password)
	if err != nil {
		h.renderLogin(w, next, "failed to hash password", true)
		return
	}
	u := store.User{
		UserID:        userID,
		Email:         email,
		PasswordHash:  hash,
		FirstName:     first,
		LastName:      last,
		PrincipalType: "User",
		PrincipalID:   userID,
		MachineCode:   machineCode,
	}
	if err := h.users.CreateUser(u); err != nil {
		msg := err.Error()
		if err == store.ErrUserExists {
			msg = "email already registered"
		}
		h.renderLogin(w, next, msg, true)
		return
	}
	if err := h.setSession(w, userID); err != nil {
		h.renderLogin(w, next, "account created but session failed; please log in", true)
		return
	}
	http.Redirect(w, r, h.redirectTarget(next), http.StatusFound)
}

// Logout clears the web session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if tok := h.sessionToken(r); tok != "" {
		_ = h.users.DeleteSession(tok)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, config.Path("/login"), http.StatusFound)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u, ok := h.CurrentUser(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	profile := u.ToProfile()
	writeJSON(w, http.StatusOK, map[string]any{
		"userId":        profile.UserID,
		"email":         profile.Email,
		"firstName":     profile.FirstName,
		"lastName":      profile.LastName,
		"principalType": profile.PrincipalType,
		"principalId":   profile.PrincipalID,
		"machineCode":   u.MachineCode,
	})
}

// LoginAPI handles POST /api/auth/login.
func (h *Handler) LoginAPI(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	u, err := h.users.GetUserByEmail(body.Email)
	if err != nil || store.CheckPassword(u.PasswordHash, body.Password) != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if err := h.setSession(w, u.UserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "session failed"})
		return
	}
	profile := u.ToProfile()
	writeJSON(w, http.StatusOK, map[string]any{
		"userId":        profile.UserID,
		"email":         profile.Email,
		"firstName":     profile.FirstName,
		"lastName":      profile.LastName,
		"principalType": profile.PrincipalType,
		"principalId":   profile.PrincipalID,
		"machineCode":   u.MachineCode,
	})
}

// CurrentUser returns the user for a valid web session cookie.
func (h *Handler) CurrentUser(r *http.Request) (*store.User, bool) {
	u, err := h.users.GetUserBySession(h.sessionToken(r))
	if err != nil || u == nil {
		return nil, false
	}
	return u, true
}

func (h *Handler) sessionToken(r *http.Request) string {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}

func (h *Handler) setSession(w http.ResponseWriter, userID string) error {
	token, err := h.users.CreateSession(userID, h.cfg.SessionTTL)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(h.cfg.SessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (h *Handler) renderLogin(w http.ResponseWriter, next, errMsg string, register bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.login.Execute(w, map[string]any{
		"Next":     next,
		"Error":    errMsg,
		"Register": register,
	})
}

func (h *Handler) redirectTarget(next string) string {
	if next != "" {
		return next
	}
	return config.Path("/account")
}

func safeNext(next string) string {
	next = strings.TrimSpace(next)
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return ""
	}
	// Only allow in-app redirects under /atlas.
	if next != config.PathPrefix && !strings.HasPrefix(next, config.PathPrefix+"/") {
		return ""
	}
	return next
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
