package releases

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestServeChannelPointer(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stable"), []byte("0.2.109"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	h := NewHandler(dir)
	r.Get("/cli/*", h.ServeHTTP)

	req := httptest.NewRequest(http.MethodGet, "/cli/stable", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("content-type=%q", ct)
	}
	if body := rec.Body.String(); body != "0.2.109" {
		t.Fatalf("body=%q", body)
	}
}

func TestServeArtifact(t *testing.T) {
	dir := t.TempDir()
	name := "grok-0.2.109-windows-x86_64.exe"
	payload := []byte("fake-binary")
	if err := os.WriteFile(filepath.Join(dir, name), payload, 0o644); err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	h := NewHandler(dir)
	r.Get("/cli/*", h.ServeHTTP)
	r.Head("/cli/*", h.ServeHTTP)

	req := httptest.NewRequest(http.MethodGet, "/cli/"+name, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status=%d", rec.Code)
	}
	got, _ := io.ReadAll(rec.Body)
	if string(got) != string(payload) {
		t.Fatalf("payload mismatch")
	}
	if cd := rec.Header().Get("Content-Disposition"); cd == "" {
		t.Fatal("missing content-disposition")
	}

	head := httptest.NewRequest(http.MethodHead, "/cli/"+name, nil)
	hrec := httptest.NewRecorder()
	r.ServeHTTP(hrec, head)
	if hrec.Code != http.StatusOK {
		t.Fatalf("head status=%d", hrec.Code)
	}
}

func TestMissingChannelIs404(t *testing.T) {
	dir := t.TempDir()
	r := chi.NewRouter()
	h := NewHandler(dir)
	r.Get("/cli/*", h.ServeHTTP)

	req := httptest.NewRequest(http.MethodGet, "/cli/stable", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestRejectPathTraversal(t *testing.T) {
	dir := t.TempDir()
	r := chi.NewRouter()
	h := NewHandler(dir)
	r.Get("/cli/*", h.ServeHTTP)

	req := httptest.NewRequest(http.MethodGet, "/cli/..%2Fsecret", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
}
