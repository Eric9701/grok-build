package releases

import (
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Handler serves CLI channel pointers and binary artifacts under /cli/*.
//
// Layout (releasesDir):
//
//	stable | alpha | enterprise   — plain-text semver
//	grok-{version}-{platform}[.exe]
//	install.ps1 / install.sh      — optional bootstrap scripts
type Handler struct {
	Dir string
}

// NewHandler returns a releases handler rooted at dir.
func NewHandler(dir string) *Handler {
	return &Handler{Dir: dir}
}

// ServeHTTP handles GET/HEAD /cli/{name}.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.Dir == "" {
		http.Error(w, "releases directory not configured", http.StatusServiceUnavailable)
		return
	}

	name := chi.URLParam(r, "*")
	if name == "" {
		name = strings.TrimPrefix(r.URL.Path, "/cli/")
	}
	name = strings.Trim(name, "/")
	if name == "" || name == "." || strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		http.NotFound(w, r)
		return
	}
	// Only allow a single path segment (basename).
	if path.Base(name) != name {
		http.NotFound(w, r)
		return
	}

	full := filepath.Join(h.Dir, name)
	absDir, err := filepath.Abs(h.Dir)
	if err != nil {
		http.Error(w, "releases dir error", http.StatusInternalServerError)
		return
	}
	absFile, err := filepath.Abs(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	rel, err := filepath.Rel(absDir, absFile)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}

	fi, err := os.Stat(absFile)
	if err != nil || fi.IsDir() {
		switch name {
		case "stable", "alpha", "enterprise":
			http.Error(w, "channel pointer not published; run scripts/publish-release.ps1", http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
		return
	}

	switch name {
	case "stable", "alpha", "enterprise":
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	case "install.ps1", "install.sh", "install-enterprise.ps1", "install-enterprise.sh":
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	default:
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
	}

	http.ServeFile(w, r, absFile)
}

// EnsureDir creates the releases directory if missing.
func EnsureDir(dir string) {
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("releases: mkdir %s: %v", dir, err)
	}
}
