// Package desktop serves the AtlasBuddy desktop self-update feed.
//
// Layout (releasesDir/desktop/{channel}/):
//
//	manifest.json                 — written last by scripts/publish-desktop.*
//	<installer / archive files>   — referenced by manifest.json, single segment names
//
// GET /desktop/{channel}/update.json renders manifest.json with absolute URLs
// for the origin the client actually reached and drops any platform whose file
// is missing or whose size differs (half-copied publish).
package desktop

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/atlas-build/atlas-server/internal/config"
	"github.com/go-chi/chi/v5"
)

// Channels lists the accepted desktop channels.
var Channels = map[string]bool{"stable": true, "alpha": true, "enterprise": true}

var fileNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ManifestPlatform is one self-update payload on disk.
type ManifestPlatform struct {
	Kind   string `json:"kind"`
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Manifest is the on-disk manifest.json shape.
type Manifest struct {
	Version      string                      `json:"version"`
	Notes        string                      `json:"notes,omitempty"`
	PubDate      string                      `json:"pub_date,omitempty"`
	MinSupported string                      `json:"min_supported,omitempty"`
	Platforms    map[string]ManifestPlatform `json:"platforms"`
	Installers   map[string]string           `json:"installers,omitempty"`
}

// FeedPlatform is one entry in update.json.
type FeedPlatform struct {
	Kind   string `json:"kind"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Feed is the update.json response.
type Feed struct {
	Version      string                  `json:"version"`
	Notes        string                  `json:"notes,omitempty"`
	PubDate      string                  `json:"pub_date,omitempty"`
	MinSupported string                  `json:"min_supported,omitempty"`
	Platforms    map[string]FeedPlatform `json:"platforms"`
	Installers   map[string]string       `json:"installers"`
}

var knownKinds = map[string]bool{"nsis": true, "app_tar_gz": true, "portable_zip": true}

// Handler serves /desktop/{channel}/{name}.
type Handler struct {
	Dir string // releasesDir root; files live in Dir/desktop/{channel}
}

// NewHandler returns a desktop feed handler rooted at releasesDir.
func NewHandler(releasesDir string) *Handler {
	return &Handler{Dir: releasesDir}
}

// ServeHTTP handles GET/HEAD /desktop/{channel}/{name}.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	channel := chi.URLParam(r, "channel")
	name := chi.URLParam(r, "name")
	if !Channels[channel] || !ValidFileName(name) {
		http.NotFound(w, r)
		return
	}
	if h.Dir == "" {
		http.Error(w, "releases directory not configured", http.StatusServiceUnavailable)
		return
	}
	dir := filepath.Join(h.Dir, "desktop", channel)

	switch name {
	case "update.json":
		h.serveFeed(w, r, dir, channel)
		return
	case "manifest.json":
		http.NotFound(w, r)
		return
	}

	full := filepath.Join(dir, name)
	fi, err := os.Stat(full)
	if err != nil || fi.IsDir() {
		http.NotFound(w, r)
		return
	}
	// File names carry the version, so the bytes never change under one URL.
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeFile(w, r, full)
}

func (h *Handler) serveFeed(w http.ResponseWriter, r *http.Request, dir, channel string) {
	m, err := ReadManifest(filepath.Join(dir, "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "desktop channel not published; run scripts/publish-desktop", http.StatusNotFound)
			return
		}
		log.Printf("desktop: read manifest %s: %v", channel, err)
		http.Error(w, "invalid desktop manifest", http.StatusInternalServerError)
		return
	}
	feed := BuildFeed(m, dir, ChannelBaseURL(r, channel))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(feed)
}

// ValidFileName reports whether name is a single safe path segment.
func ValidFileName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.Contains(name, "..") && fileNameRe.MatchString(name)
}

// ReadManifest loads and decodes manifest.json.
func ReadManifest(path string) (Manifest, error) {
	var m Manifest
	b, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

// BuildFeed converts the on-disk manifest into update.json, keeping only
// platforms whose file exists with the recorded size.
func BuildFeed(m Manifest, dir, baseURL string) Feed {
	feed := Feed{
		Version:      m.Version,
		Notes:        m.Notes,
		PubDate:      m.PubDate,
		MinSupported: m.MinSupported,
		Platforms:    map[string]FeedPlatform{},
		Installers:   map[string]string{},
	}
	for key, p := range m.Platforms {
		if !knownKinds[p.Kind] || !ValidFileName(p.File) || len(p.SHA256) != 64 || p.Size <= 0 {
			log.Printf("desktop: drop %s: invalid manifest entry", key)
			continue
		}
		fi, err := os.Stat(filepath.Join(dir, p.File))
		if err != nil || fi.IsDir() || fi.Size() != p.Size {
			log.Printf("desktop: drop %s: %s missing or size mismatch", key, p.File)
			continue
		}
		feed.Platforms[key] = FeedPlatform{
			Kind:   p.Kind,
			URL:    baseURL + "/" + p.File,
			SHA256: strings.ToLower(p.SHA256),
			Size:   p.Size,
		}
	}
	for key, file := range m.Installers {
		if !ValidFileName(file) {
			continue
		}
		if fi, err := os.Stat(filepath.Join(dir, file)); err != nil || fi.IsDir() {
			continue
		}
		feed.Installers[key] = baseURL + "/" + file
	}
	return feed
}

// ChannelBaseURL is the absolute URL of /desktop/{channel} as the client
// reached it (reverse-proxy headers first), so download URLs share its origin.
func ChannelBaseURL(r *http.Request, channel string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if p := firstHeader(r, "X-Forwarded-Proto"); p == "http" || p == "https" {
		scheme = p
	}
	host := firstHeader(r, "X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host + config.Path("/desktop/"+channel)
}

func firstHeader(r *http.Request, key string) string {
	v := r.Header.Get(key)
	if i := strings.IndexByte(v, ','); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}
