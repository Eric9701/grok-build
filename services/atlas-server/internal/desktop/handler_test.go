package desktop

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

const fakeSHA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func newRouter(root string) http.Handler {
	r := chi.NewRouter()
	h := NewHandler(root)
	r.Get("/atlas/desktop/{channel}/{name}", h.ServeHTTP)
	r.Head("/atlas/desktop/{channel}/{name}", h.ServeHTTP)
	return r
}

func publish(t *testing.T, root string, m Manifest, files map[string]string) {
	t.Helper()
	dir := filepath.Join(root, "desktop", "stable")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func getFeed(t *testing.T, h http.Handler, host string, hdr map[string]string) (int, Feed) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/atlas/desktop/stable/update.json", nil)
	req.Host = host
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var f Feed
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &f); err != nil {
			t.Fatalf("decode: %v body=%q", err, rec.Body.String())
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
			t.Fatalf("cache-control=%q", cc)
		}
	}
	return rec.Code, f
}

func TestFeedUsesRequestOriginAndDropsBrokenPlatforms(t *testing.T) {
	root := t.TempDir()
	publish(t, root, Manifest{
		Version: "0.2.43",
		Platforms: map[string]ManifestPlatform{
			"windows-x86_64": {Kind: "nsis", File: "AtlasBuddy_0.2.43_x64-setup.exe", SHA256: fakeSHA, Size: 5},
			"darwin-aarch64": {Kind: "app_tar_gz", File: "AtlasBuddy_0.2.43_aarch64.app.tar.gz", SHA256: fakeSHA, Size: 999},
			"darwin-x86_64":  {Kind: "app_tar_gz", File: "missing.app.tar.gz", SHA256: fakeSHA, Size: 3},
		},
		Installers: map[string]string{
			"windows-x86_64": "AtlasBuddy_0.2.43_x64-setup.exe",
			"darwin-aarch64": "missing.dmg",
		},
	}, map[string]string{
		"AtlasBuddy_0.2.43_x64-setup.exe":      "hello",
		"AtlasBuddy_0.2.43_aarch64.app.tar.gz": "short",
	})

	code, f := getFeed(t, newRouter(root), "10.0.0.5:22255", nil)
	if code != http.StatusOK {
		t.Fatalf("status=%d", code)
	}
	if f.Version != "0.2.43" {
		t.Fatalf("version=%q", f.Version)
	}
	win, ok := f.Platforms["windows-x86_64"]
	if !ok {
		t.Fatalf("windows platform missing: %+v", f.Platforms)
	}
	want := "http://10.0.0.5:22255/atlas/desktop/stable/AtlasBuddy_0.2.43_x64-setup.exe"
	if win.URL != want || win.Size != 5 || win.SHA256 != fakeSHA {
		t.Fatalf("windows=%+v", win)
	}
	if _, ok := f.Platforms["darwin-aarch64"]; ok {
		t.Fatal("size mismatch must be dropped")
	}
	if _, ok := f.Platforms["darwin-x86_64"]; ok {
		t.Fatal("missing file must be dropped")
	}
	if f.Installers["windows-x86_64"] != want || len(f.Installers) != 1 {
		t.Fatalf("installers=%+v", f.Installers)
	}
}

func TestFeedHonorsForwardedHeaders(t *testing.T) {
	root := t.TempDir()
	publish(t, root, Manifest{
		Version: "0.2.43",
		Platforms: map[string]ManifestPlatform{
			"windows-x86_64": {Kind: "nsis", File: "a.exe", SHA256: fakeSHA, Size: 1},
		},
	}, map[string]string{"a.exe": "x"})

	_, f := getFeed(t, newRouter(root), "127.0.0.1:22255", map[string]string{
		"X-Forwarded-Proto": "https",
		"X-Forwarded-Host":  "atlas.corp.example",
	})
	if got := f.Platforms["windows-x86_64"].URL; got != "https://atlas.corp.example/atlas/desktop/stable/a.exe" {
		t.Fatalf("url=%q", got)
	}
}

func TestFeedNotPublished(t *testing.T) {
	code, _ := getFeed(t, newRouter(t.TempDir()), "h", nil)
	if code != http.StatusNotFound {
		t.Fatalf("status=%d", code)
	}
}

func TestRejectsBadChannelAndNames(t *testing.T) {
	root := t.TempDir()
	publish(t, root, Manifest{Version: "1.0.0"}, map[string]string{"ok.exe": "x"})
	h := newRouter(root)
	for _, p := range []string{
		"/atlas/desktop/nightly/update.json",
		"/atlas/desktop/stable/manifest.json",
		"/atlas/desktop/stable/..",
		"/atlas/desktop/stable/a..b",
		"/atlas/desktop/stable/%2e%2e%2fsecret",
		"/atlas/desktop/stable/missing.exe",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status=%d", p, rec.Code)
		}
	}
}

func TestServesPayloadWithRange(t *testing.T) {
	root := t.TempDir()
	publish(t, root, Manifest{Version: "1.0.0"}, map[string]string{"pkg.exe": "0123456789"})
	req := httptest.NewRequest(http.MethodGet, "/atlas/desktop/stable/pkg.exe", nil)
	req.Header.Set("Range", "bytes=4-")
	rec := httptest.NewRecorder()
	newRouter(root).ServeHTTP(rec, req)
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status=%d", rec.Code)
	}
	if rec.Body.String() != "456789" {
		t.Fatalf("body=%q", rec.Body.String())
	}
}
