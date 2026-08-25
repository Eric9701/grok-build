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
