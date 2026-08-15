package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/atlas-build/atlas-server/internal/config"
	"github.com/atlas-build/atlas-server/internal/store"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const deviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"

// SessionUser resolves the logged-in web account (device approval gate).
type SessionUser interface {
	CurrentUser(r *http.Request) (*store.User, bool)
}

// Handler serves OAuth/OIDC endpoints expected by the Atlas CLI.
type Handler struct {
	cfg     config.Config
	store   store.Store
	users   store.UserStore
	session SessionUser
	page    *template.Template
}

// NewHandler constructs an auth Handler.
func NewHandler(cfg config.Config, st store.Store, users store.UserStore, session SessionUser, page *template.Template) *Handler {
	return &Handler{cfg: cfg, store: st, users: users, session: session, page: page}
}

// Discovery returns OpenID Provider Metadata.
func (h *Handler) Discovery(w http.ResponseWriter, r *http.Request) {
	base := h.cfg.PublicBaseURL
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                base,
		"authorization_endpoint":                base + "/authorize",
		"token_endpoint":                        base + "/oauth2/token",
		"device_authorization_endpoint":         base + "/oauth2/device/code",
		"userinfo_endpoint":                     base + "/v1/user",
		"jwks_uri":                              base + "/.well-known/jwks.json",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token", deviceGrantType},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"HS256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"scopes_supported": []string{
			"openid", "profile", "email", "offline_access",
			"grok-cli:access", "api:access",
		},
	})
}

// Authorize redirects legacy /authorize (authorization-code) browser hits to the
// machine-code device login page. Atlas CLI login uses the device flow by
// default; this keeps bookmarks and discovery authorization_endpoint links useful.
func (h *Handler) Authorize(w http.ResponseWriter, r *http.Request) {
	target := config.Path("/oauth2/device")
	if userCode := r.URL.Query().Get("user_code"); userCode != "" {
		target += "?user_code=" + userCode
	}
	http.Redirect(w, r, target, http.StatusFound)
}

// RequestDeviceCode handles POST /oauth2/device/code.
func (h *Handler) RequestDeviceCode(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	clientID := r.Form.Get("client_id")
	if clientID == "" {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", "client_id required")
		return
	}
	scope := r.Form.Get("scope")

	deviceCode, err := store.RandomHex(24)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "failed to mint device_code")
		return
	}
	userCode, err := store.RandomUserCode()
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", "failed to mint user_code")
		return
	}

	verifyURI := h.cfg.PublicBaseURL + "/oauth2/device"
	verifyComplete := verifyURI + "?user_code=" + userCode
	expiresIn := int64(h.cfg.DeviceTTL.Seconds())
	interval := 5

	sess := store.DeviceSession{
		DeviceCode:              deviceCode,
		UserCode:                userCode,
		ClientID:                clientID,
		Scope:                   scope,
		Status:                  store.DevicePending,
		ExpiresAt:               time.Now().Add(h.cfg.DeviceTTL),
		IntervalSecs:            interval,
		VerificationURI:         verifyURI,
		VerificationURIComplete: verifyComplete,
	}
	if err := h.store.CreateDevice(sess); err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"device_code":               deviceCode,
		"user_code":                 userCode,
		"verification_uri":          verifyURI,
		"verification_uri_complete": verifyComplete,
		"expires_in":                expiresIn,
		"interval":                  interval,
	})
}

// DevicePage serves the human verification UI (requires web login + machine code).
func (h *Handler) DevicePage(w http.ResponseWriter, r *http.Request) {
	userCode := r.URL.Query().Get("user_code")
	next := config.Path("/oauth2/device")
	if userCode != "" {
		next += "?user_code=" + userCode
	}
	loggedIn, _ := h.session.CurrentUser(r)
	if loggedIn == nil {
		http.Redirect(w, r, config.Path("/login")+"?next="+next, http.StatusFound)
		return
	}

	data := map[string]any{
		"UserCode":    userCode,
		"MachineCode": loggedIn.MachineCode,
		"Email":       loggedIn.Email,
		"Error":       "",
		"Success":     false,
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			data["Error"] = "invalid form"
			h.renderDevice(w, data)
			return
		}
		userCode = r.Form.Get("user_code")
		machineCode := r.Form.Get("machine_code")
		action := r.Form.Get("action")
		data["UserCode"] = userCode
		switch action {
		case "deny":
			_ = h.store.DenyByUserCode(userCode)
			data["Error"] = "Authorization denied."
		default:
			if loggedIn.MachineCode == "" {
				data["Error"] = "your account has no machine code; contact an administrator"
			} else if strings.ToUpper(strings.TrimSpace(machineCode)) != strings.ToUpper(strings.TrimSpace(loggedIn.MachineCode)) {
				data["Error"] = "machine code does not match your account"
			} else if err := h.store.ApproveByUserCode(userCode, loggedIn.UserID, loggedIn.Email); err != nil {
				data["Error"] = err.Error()
			} else {
				data["Success"] = true
			}
		}
	}
	h.renderDevice(w, data)
}

func (h *Handler) renderDevice(w http.ResponseWriter, data map[string]any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.page.Execute(w, data)
}

// Token handles POST /oauth2/token (device_code + refresh_token).
func (h *Handler) Token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	grant := r.Form.Get("grant_type")
	switch grant {
	case deviceGrantType:
		h.tokenDevice(w, r)
	case "refresh_token":
		h.tokenRefresh(w, r)
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type", grant)
	}
}

func (h *Handler) tokenDevice(w http.ResponseWriter, r *http.Request) {
	deviceCode := r.Form.Get("device_code")
	clientID := r.Form.Get("client_id")
	sess, err := h.store.GetByDeviceCode(deviceCode)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "unknown device_code")
		return
	}
	if clientID != "" && sess.ClientID != clientID {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client", "client_id mismatch")
		return
	}
	switch sess.Status {
	case store.DevicePending:
		writeOAuthError(w, http.StatusBadRequest, "authorization_pending", "waiting for user")
		return
	case store.DeviceDenied:
		writeOAuthError(w, http.StatusBadRequest, "access_denied", "user denied")
		return
	case store.DeviceExpired:
		writeOAuthError(w, http.StatusBadRequest, "expired_token", "device code expired")
		return
	case store.DeviceConsumed:
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "already used")
		return
	case store.DeviceApproved:
		// continue
	default:
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "unexpected status")
		return
	}

	tokens, err := h.issueTokensForUser(sess.UserID, sess.ClientID, sess.Scope)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	_ = h.store.MarkConsumed(deviceCode)
	writeJSON(w, http.StatusOK, tokens)
}

func (h *Handler) tokenRefresh(w http.ResponseWriter, r *http.Request) {
	refresh := r.Form.Get("refresh_token")
	rec, err := h.store.GetRefresh(refresh)
	if err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant", err.Error())
		return
	}
	_ = h.store.RevokeRefresh(refresh)
	tokens, err := h.issueTokensForUser(rec.UserID, rec.ClientID, rec.Scope)
	if err != nil {
		writeOAuthError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

type atlasClaims struct {
	PrincipalType string `json:"principal_type,omitempty"`
	PrincipalID   string `json:"principal_id,omitempty"`
	Email         string `json:"email,omitempty"`
	jwt.RegisteredClaims
}

func (h *Handler) issueTokensForUser(userID, clientID, scope string) (*tokenResponse, error) {
	email := ""
	firstName := "Atlas"
	lastName := "Dev"
	principalType := "User"
	principalID := userID
	if h.users != nil {
		if u, err := h.users.GetUserByID(userID); err == nil && u != nil {
			email = u.Email
			firstName = u.FirstName
			lastName = u.LastName
			principalType = u.PrincipalType
			principalID = u.PrincipalID
		}
	}
	if email == "" {
		email = h.cfg.DefaultEmail
	}
	if userID == "" {
		userID = h.cfg.DefaultUserID
		principalID = userID
	}
	return h.issueTokens(userID, email, firstName, lastName, principalType, principalID, clientID, scope)
}

func (h *Handler) issueTokens(userID, email, firstName, lastName, principalType, principalID, clientID, scope string) (*tokenResponse, error) {
	now := time.Now()
	accessExp := now.Add(h.cfg.AccessTokenTTL)
	claims := atlasClaims{
		PrincipalType: principalType,
		PrincipalID:   principalID,
		Email:         email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    h.cfg.PublicBaseURL,
			Audience:  []string{clientID},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExp),
			ID:        uuid.NewString(),
		},
	}
	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(h.cfg.JWTSecret)
	if err != nil {
		return nil, err
	}

	idClaims := atlasClaims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			Issuer:    h.cfg.PublicBaseURL,
			Audience:  []string{clientID},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExp),
		},
	}
	idToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, idClaims).SignedString(h.cfg.JWTSecret)
	if err != nil {
		return nil, err
	}

	refresh, err := randomToken(32)
	if err != nil {
		return nil, err
	}
	_ = h.store.SaveRefresh(store.RefreshRecord{
		Token:     refresh,
		UserID:    userID,
		Email:     email,
		ClientID:  clientID,
		Scope:     scope,
		ExpiresAt: now.Add(h.cfg.RefreshTTL),
	})

	return &tokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(h.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refresh,
		Scope:        scope,
		IDToken:      idToken,
	}, nil
}

// ParseBearerUser extracts user id/email from a Bearer access token.
func (h *Handler) ParseBearerUser(r *http.Request) (userID, email string, ok bool) {
	authz := r.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
		return "", "", false
	}
	raw := strings.TrimSpace(authz[7:])
	claims := &atlasClaims{}
	tok, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		return h.cfg.JWTSecret, nil
	})
	if err != nil || !tok.Valid {
		return "", "", false
	}
	return claims.Subject, claims.Email, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeOAuthError(w http.ResponseWriter, status int, code, desc string) {
	writeJSON(w, status, map[string]string{
		"error":             code,
		"error_description": desc,
	})
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// DiscardBody drains request body (unused helper for future authorize).
func DiscardBody(r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
}
