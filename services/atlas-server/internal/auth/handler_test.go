package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/atlas-build/atlas-server/internal/config"
	"github.com/atlas-build/atlas-server/internal/store"
)

func testAuthHandler(st store.Store) *Handler {
	return NewHandler(config.Config{
		PublicBaseURL:  "http://atlas.example/atlas",
		JWTSecret:      []byte("test-secret"),
		AccessTokenTTL: time.Hour,
		RefreshTTL:     24 * time.Hour,
		DefaultUserID:  "atlas-local-user",
		DefaultEmail:   "dev@atlas.local",
	}, st, nil, nil, nil)
}

func postToken(t *testing.T, h *Handler, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/atlas/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.Token(rec, req)
	return rec
}

func TestTokenRefreshRotatesAndKeepsUser(t *testing.T) {
	st := store.NewMemoryStore()
	old := store.RefreshRecord{
		Token:     "old-rt",
		UserID:    "zhangyufeng",
		Email:     "zhangyufeng@imyai.cn",
		ClientID:  "atlas-cli",
		Scope:     "openid offline_access",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := st.SaveRefresh(old); err != nil {
		t.Fatal(err)
	}
	h := testAuthHandler(st)
	rec := postToken(t, h, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"old-rt"},
		"client_id":     {"atlas-cli"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body tokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.AccessToken == "" || body.RefreshToken == "" || body.RefreshToken == "old-rt" {
		t.Fatalf("tokens %+v", body)
	}
	if _, err := st.GetRefresh("old-rt"); !errors.Is(err, store.ErrRefreshNotFound) {
		t.Fatalf("old refresh still valid: %v", err)
	}
	got, err := st.GetRefresh(body.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != "zhangyufeng" {
		t.Fatalf("rotated user %q", got.UserID)
	}
}

func TestTokenRefreshUnknownIsInvalidGrant(t *testing.T) {
	h := testAuthHandler(store.NewMemoryStore())
	rec := postToken(t, h, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"missing"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "invalid_grant" {
		t.Fatalf("error %q", body["error"])
	}
}

type saveFailStore struct {
	*store.MemoryStore
}

func (s saveFailStore) SaveRefresh(store.RefreshRecord) error {
	return errors.New("db down")
}

type memUsers struct {
	byEmail map[string]*store.User
}

func (m *memUsers) Migrate() error              { return nil }
func (m *memUsers) CreateUser(store.User) error { return nil }
func (m *memUsers) EnsureBootstrapUser(store.User, string) error {
	return nil
}
func (m *memUsers) CreateSession(string, time.Duration) (string, error) {
	return "", errors.New("unused")
}
func (m *memUsers) GetUserBySession(string) (*store.User, error) {
	return nil, store.ErrSessionNotFound
}
func (m *memUsers) DeleteSession(string) error { return nil }
func (m *memUsers) GetUserByMachineCode(string) (*store.User, error) {
	return nil, store.ErrUserNotFound
}
func (m *memUsers) GetUserByEmail(email string) (*store.User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return nil, store.ErrUserNotFound
	}
	cp := *u
	return &cp, nil
}
func (m *memUsers) GetUserByID(id string) (*store.User, error) {
	for _, u := range m.byEmail {
		if u.UserID == id {
			cp := *u
			return &cp, nil
		}
	}
	return nil, store.ErrUserNotFound
}

func passwordHandler(t *testing.T, email, password, machine string) *Handler {
	t.Helper()
	hash, err := store.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	h := testAuthHandler(store.NewMemoryStore())
	h.users = &memUsers{byEmail: map[string]*store.User{
		email: {
			UserID:       "user-1",
			Email:        email,
			PasswordHash: hash,
			MachineCode:  machine,
		},
	}}
	return h
}

func passwordForm(email, password, machine string) url.Values {
	return url.Values{
		"grant_type":   {"password"},
		"client_id":    {"atlas-cli"},
		"username":     {email},
		"password":     {password},
		"machine_code": {machine},
		"scope":        {"openid offline_access"},
	}
}

func oauthError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body["error_description"]
}

func TestTokenPasswordIssuesSessionTokens(t *testing.T) {
	h := passwordHandler(t, "dev@atlas.local", "atlas123", "ABCD-EFGH")
	rec := postToken(t, h, passwordForm("dev@atlas.local", "atlas123", "abcd-efgh"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var body tokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.AccessToken == "" || body.RefreshToken == "" || body.IDToken == "" {
		t.Fatalf("tokens %+v", body)
	}
	got, err := h.store.GetRefresh(body.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != "user-1" || got.Scope != "openid offline_access" {
		t.Fatalf("refresh %+v", got)
	}
	if rec.Result().Header.Get("Set-Cookie") != "" {
		t.Fatal("password grant must not set a web session cookie")
	}
}

func TestTokenPasswordFailuresShareOneMessage(t *testing.T) {
	h := passwordHandler(t, "dev@atlas.local", "atlas123", "ABCD-EFGH")
	cases := []url.Values{
		passwordForm("missing@atlas.local", "atlas123", "ABCD-EFGH"),
		passwordForm("dev@atlas.local", "wrong", "ABCD-EFGH"),
		passwordForm("dev@atlas.local", "atlas123", "ZZZZ-ZZZZ"),
		passwordForm("dev@atlas.local", "atlas123", ""),
	}
	for _, form := range cases {
		rec := postToken(t, h, form)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
		}
		if got := oauthError(t, rec); got != passwordLoginError {
			t.Fatalf("description %q", got)
		}
	}
}

func TestTokenPasswordLocksThenResets(t *testing.T) {
	h := passwordHandler(t, "dev@atlas.local", "atlas123", "ABCD-EFGH")
	for i := 0; i < passwordFailLimit-1; i++ {
		rec := postToken(t, h, passwordForm("dev@atlas.local", "wrong", "ABCD-EFGH"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("fail %d status %d", i, rec.Code)
		}
	}
	ok := postToken(t, h, passwordForm("dev@atlas.local", "atlas123", "ABCD-EFGH"))
	if ok.Code != http.StatusOK {
		t.Fatalf("success before lock: %d %s", ok.Code, ok.Body.String())
	}
	for i := 0; i < passwordFailLimit; i++ {
		_ = postToken(t, h, passwordForm("dev@atlas.local", "wrong", "ABCD-EFGH"))
	}
	locked := postToken(t, h, passwordForm("dev@atlas.local", "atlas123", "ABCD-EFGH"))
	if locked.Code != http.StatusBadRequest || oauthError(t, locked) != passwordLoginError {
		t.Fatalf("locked status %d body %s", locked.Code, locked.Body.String())
	}
}

func TestDiscoveryListsPasswordGrant(t *testing.T) {
	h := testAuthHandler(store.NewMemoryStore())
	req := httptest.NewRequest(http.MethodGet, "/atlas/.well-known/openid-configuration", nil)
	rec := httptest.NewRecorder()
	h.Discovery(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	grants, _ := body["grant_types_supported"].([]any)
	found := false
	for _, g := range grants {
		if g == passwordGrantType {
			found = true
		}
	}
	if !found {
		t.Fatalf("grants %#v", body["grant_types_supported"])
	}
}

func TestTokenRefreshKeepsOldTokenWhenPersistFails(t *testing.T) {
	mem := store.NewMemoryStore()
	if err := mem.SaveRefresh(store.RefreshRecord{
		Token:     "old-rt",
		UserID:    "u1",
		ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	h := testAuthHandler(saveFailStore{MemoryStore: mem})
	rec := postToken(t, h, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {"old-rt"},
	})
	if rec.Code != http.StatusInternalServerError {
		body, _ := io.ReadAll(rec.Result().Body)
		t.Fatalf("status %d body %s", rec.Code, body)
	}
	if _, err := mem.GetRefresh("old-rt"); err != nil {
		t.Fatalf("old refresh must remain after persist failure: %v", err)
	}
}
