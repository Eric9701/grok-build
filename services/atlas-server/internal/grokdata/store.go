package grokdata

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Store reads probe caches from DownloadDir and optional ~/.grok content.
type Store struct {
	Home        string // auth.json / bundled / skills
	DownloadDir string // probe_* files (default: cwd/download)

	mu    sync.Mutex
	token string
}

// New constructs a Store.
func New(grokHome, downloadDir string) *Store {
	if downloadDir == "" {
		downloadDir = "download"
	}
	return &Store{Home: grokHome, DownloadDir: downloadDir}
}

// Downloads returns the probe/data directory (cwd/download by default).
func (s *Store) Downloads() string { return s.DownloadDir }

// Bundled returns ~/.grok/bundled.
func (s *Store) Bundled() string { return filepath.Join(s.Home, "bundled") }

// Skills returns ~/.grok/skills (user skills).
func (s *Store) Skills() string { return filepath.Join(s.Home, "skills") }

// ReadProbeJSON loads a previously captured real-proxy response if present.
func (s *Store) ReadProbeJSON(name string) ([]byte, bool) {
	candidates := []string{
		filepath.Join(s.Downloads(), name),
		filepath.Join(s.Downloads(), name+".json"),
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil && len(b) > 0 {
			return b, true
		}
	}
	return nil, false
}

// ReadProbeFile reads a binary probe artifact (e.g. bundle archive).
func (s *Store) ReadProbeFile(name string) ([]byte, bool) {
	p := filepath.Join(s.Downloads(), name)
	b, err := os.ReadFile(p)
	if err != nil || len(b) == 0 {
		return nil, false
	}
	return b, true
}

// AuthToken returns the first OAuth access token from auth.json (cached).
func (s *Store) AuthToken() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" {
		return s.token, nil
	}
	raw, err := os.ReadFile(filepath.Join(s.Home, "auth.json"))
	if err != nil {
		return "", err
	}
	var doc map[string]map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", err
	}
	for _, entry := range doc {
		if v, ok := entry["key"].(string); ok && v != "" {
			s.token = v
			return v, nil
		}
		if v, ok := entry["access_token"].(string); ok && v != "" {
			s.token = v
			return v, nil
		}
	}
	return "", fmt.Errorf("no token in %s/auth.json", s.Home)
}

// BuildSubagentBundleJSON builds the legacy JSON bundle from bundled/ (+ user skills overlay).
func (s *Store) BuildSubagentBundleJSON() ([]byte, error) {
	out := map[string]any{
		"version":  "atlas-local-from-grok-home",
		"personas": map[string]string{},
		"roles":    map[string]string{},
		"agents":   map[string]string{},
		"skills":   map[string]string{},
	}
	personas := out["personas"].(map[string]string)
	roles := out["roles"].(map[string]string)
	agents := out["agents"].(map[string]string)
	skills := out["skills"].(map[string]string)

	loadNamed := func(dir string, into map[string]string, ext string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ext) {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return err
			}
			into[strings.TrimSuffix(name, ext)] = string(b)
		}
		return nil
	}

	bundled := s.Bundled()
	if err := loadNamed(filepath.Join(bundled, "personas"), personas, ".toml"); err != nil {
		return nil, err
	}
	if err := loadNamed(filepath.Join(bundled, "roles"), roles, ".toml"); err != nil {
		return nil, err
	}
	if err := loadNamed(filepath.Join(bundled, "agents"), agents, ".md"); err != nil {
		return nil, err
	}

	loadSkills := func(root string) error {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			skillMD := filepath.Join(root, e.Name(), "SKILL.md")
			b, err := os.ReadFile(skillMD)
			if err != nil {
				continue
			}
			skills[e.Name()] = string(b)
		}
		return nil
	}
	_ = loadSkills(filepath.Join(bundled, "skills"))
	_ = loadSkills(s.Skills()) // user skills override bundled names

	return json.Marshal(out)
}

// BuildBundleArchive builds a gzipped tar matching CLI extract layout from bundled/.
func (s *Store) BuildBundleArchive() ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	writeBytes := func(name string, body []byte) error {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err := tw.Write(body)
		return err
	}

	meta, _ := json.Marshal(map[string]string{"version": "archive-v1"})
	if err := writeBytes("bundle.json", meta); err != nil {
		return nil, err
	}

	root := s.Bundled()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		// Map filesystem layout into archive layout expected by CLI.
		var name string
		switch {
		case strings.HasPrefix(rel, "skills/"):
			name = rel
		case strings.HasPrefix(rel, "agents/"):
			name = "subagents/" + rel
		case strings.HasPrefix(rel, "personas/"):
			name = "subagents/" + rel
		case strings.HasPrefix(rel, "roles/"):
			name = "subagents/" + rel
		case rel == "manifest.json":
			return nil // CLI writes its own manifest after extract
		default:
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return writeBytes(name, body)
	})
	if err != nil {
		return nil, err
	}

	// Overlay user skills last so they win inside the archive too.
	_ = filepath.WalkDir(s.Skills(), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(s.Skills(), path)
		if err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return writeBytes("skills/"+filepath.ToSlash(rel), body)
	})

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SaveProbe writes bytes into DownloadDir for later local-first serving.
func (s *Store) SaveProbe(name string, body []byte) error {
	if err := os.MkdirAll(s.DownloadDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.DownloadDir, name), body, 0o600)
}

// CopyReader is a tiny helper for callers.
func CopyReader(r io.Reader) ([]byte, error) { return io.ReadAll(r) }
