package store

import (
	"errors"
	"testing"
	"time"
)

func TestMemoryStoreRefreshLifecycle(t *testing.T) {
	m := NewMemoryStore()
	rec := RefreshRecord{
		Token:     "rt-1",
		UserID:    "u1",
		Email:     "u1@atlas.local",
		ClientID:  "atlas-cli",
		Scope:     "openid",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := m.SaveRefresh(rec); err != nil {
		t.Fatal(err)
	}
	got, err := m.GetRefresh("rt-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != "u1" || got.Email != rec.Email {
		t.Fatalf("got %+v", got)
	}
	if err := m.RevokeRefresh("rt-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetRefresh("rt-1"); !errors.Is(err, ErrRefreshNotFound) {
		t.Fatalf("after revoke: %v", err)
	}
}

func TestMemoryStoreRefreshExpired(t *testing.T) {
	m := NewMemoryStore()
	if err := m.SaveRefresh(RefreshRecord{
		Token:     "rt-old",
		UserID:    "u1",
		ExpiresAt: time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetRefresh("rt-old"); !errors.Is(err, ErrRefreshExpired) {
		t.Fatalf("expired: %v", err)
	}
}

func TestAuthStoreRefreshGoesToPersister(t *testing.T) {
	backend := NewMemoryStore()
	s := newAuthStore(backend)
	rec := RefreshRecord{
		Token:     "rt-persist",
		UserID:    "u1",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := s.SaveRefresh(rec); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.GetRefresh("rt-persist"); err != nil {
		t.Fatalf("persister missing token: %v", err)
	}
	// Device-code memory must not hold the refresh token.
	if _, err := s.(*authStore).MemoryStore.GetRefresh("rt-persist"); !errors.Is(err, ErrRefreshNotFound) {
		t.Fatalf("refresh leaked into device memory: %v", err)
	}
	got, err := s.GetRefresh("rt-persist")
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != "u1" {
		t.Fatalf("got %+v", got)
	}
	if err := s.RevokeRefresh("rt-persist"); err != nil {
		t.Fatal(err)
	}
	if _, err := backend.GetRefresh("rt-persist"); !errors.Is(err, ErrRefreshNotFound) {
		t.Fatalf("persister still has token: %v", err)
	}
}
