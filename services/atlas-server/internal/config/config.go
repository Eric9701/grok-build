package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// PathPrefix is the HTTP mount path for all atlas-server routes
// (CLI proxy base becomes {host}{PathPrefix}/v1).
const PathPrefix = "/atlas"

// Path joins PathPrefix with a root-relative path (e.g. Path("/v1/traces")).
func Path(p string) string {
	if p == "" || p == "/" {
		return PathPrefix
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return PathPrefix + p
}

// Config holds runtime settings for atlas-server.
type Config struct {
	Addr           string
	PublicBaseURL  string
	JWTSecret      []byte
	AccessTokenTTL time.Duration
	RefreshTTL     time.Duration
	DeviceTTL      time.Duration
	DefaultUserID  string
	DefaultEmail   string

	GrokHome        string
	DownloadDir     string
	ReleasesDir     string
	UpstreamBase    string
	UpstreamEnabled bool

	MySQLDSN          string
	SessionTTL        time.Duration
	BootstrapPassword string
}

// fileConfig is the on-disk TOML shape. Environment variables override file values.
type fileConfig struct {
	Server struct {
		Addr          string `toml:"addr"`
		PublicBaseURL string `toml:"public_base_url"`
		JWTSecret     string `toml:"jwt_secret"`
	} `toml:"server"`

	Auth struct {
		AccessTokenTTL string `toml:"access_token_ttl"`
		RefreshTTL     string `toml:"refresh_ttl"`
		DeviceTTL      string `toml:"device_ttl"`
		SessionTTL     string `toml:"session_ttl"`
	} `toml:"auth"`

	Bootstrap struct {
		UserID   string `toml:"user_id"`
		Email    string `toml:"email"`
		Password string `toml:"password"`
	} `toml:"bootstrap"`

	MySQL mysqlFile `toml:"mysql"`

	Data struct {
		GrokHome    string `toml:"grok_home"`
		DownloadDir string `toml:"download_dir"`
		ReleasesDir string `toml:"releases_dir"`
	} `toml:"data"`

	Upstream *upstreamFile `toml:"upstream"`
}

type upstreamFile struct {
	Enabled bool   `toml:"enabled"`
	BaseURL string `toml:"base_url"`
}

type mysqlFile struct {
	DSN      string `toml:"dsn"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	Database string `toml:"database"`
}

// Load reads defaults, merges an optional TOML config file, then applies env overrides.
//
// Config file path: ATLAS_CONFIG, or the first existing file among:
//   - ./atlas-server.toml
//   - ./config/atlas-server.toml
//   - <executable-dir>/atlas-server.toml
func Load() Config {
	home, _ := os.UserHomeDir()
	cfg := Config{
		Addr:              ":22255",
		PublicBaseURL:     "http://10.218.220.237:22255/atlas",
		JWTSecret:         []byte("atlas-dev-secret-change-me"),
		AccessTokenTTL:    time.Hour,
		RefreshTTL:        30 * 24 * time.Hour,
		DeviceTTL:         15 * time.Minute,
		SessionTTL:        7 * 24 * time.Hour,
		DefaultUserID:     "atlas-local-user",
		DefaultEmail:      "dev@atlas.local",
		BootstrapPassword: "atlas-dev",
		GrokHome:          filepath.Join(home, ".grok"),
		DownloadDir:       resolveDownloadDir(""),
		ReleasesDir:       "",
		UpstreamBase:      "https://cli-chat-proxy.grok.com/v1",
		UpstreamEnabled:   true,
		MySQLDSN:          "atlas:atlas@tcp(127.0.0.1:3306)/atlas?parseTime=true&charset=utf8mb4",
	}

	if path := resolveConfigPath(); path != "" {
		if err := mergeFile(&cfg, path); err != nil {
			fmt.Fprintf(os.Stderr, "warning: load config %s: %v\n", path, err)
		} else {
			fmt.Fprintf(os.Stderr, "loaded config: %s\n", path)
		}
	}

	applyEnvOverrides(&cfg)
	cfg.PublicBaseURL = trimSlash(cfg.PublicBaseURL)
	cfg.UpstreamBase = trimSlash(cfg.UpstreamBase)
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = resolveDownloadDir("")
	}
	if cfg.ReleasesDir == "" {
		cfg.ReleasesDir = resolveReleasesDir("")
	}
	return cfg
}

func mergeFile(cfg *Config, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var f fileConfig
	if err := toml.Unmarshal(b, &f); err != nil {
		return err
	}

	if f.Server.Addr != "" {
		cfg.Addr = f.Server.Addr
	}
	if f.Server.PublicBaseURL != "" {
		cfg.PublicBaseURL = f.Server.PublicBaseURL
	}
	if f.Server.JWTSecret != "" {
		cfg.JWTSecret = []byte(f.Server.JWTSecret)
	}

	if d, ok := parseDuration(f.Auth.AccessTokenTTL); ok {
		cfg.AccessTokenTTL = d
	}
	if d, ok := parseDuration(f.Auth.RefreshTTL); ok {
		cfg.RefreshTTL = d
	}
	if d, ok := parseDuration(f.Auth.DeviceTTL); ok {
		cfg.DeviceTTL = d
	}
	if d, ok := parseDuration(f.Auth.SessionTTL); ok {
		cfg.SessionTTL = d
	}

	if f.Bootstrap.UserID != "" {
		cfg.DefaultUserID = f.Bootstrap.UserID
	}
	if f.Bootstrap.Email != "" {
		cfg.DefaultEmail = f.Bootstrap.Email
	}
	if f.Bootstrap.Password != "" {
		cfg.BootstrapPassword = f.Bootstrap.Password
	}

	if dsn := mysqlDSNFromFile(f.MySQL); dsn != "" {
		cfg.MySQLDSN = dsn
	}

	if f.Data.GrokHome != "" {
		cfg.GrokHome = f.Data.GrokHome
	}
	if f.Data.DownloadDir != "" {
		cfg.DownloadDir = f.Data.DownloadDir
	}
	if f.Data.ReleasesDir != "" {
		cfg.ReleasesDir = f.Data.ReleasesDir
	}

	if f.Upstream != nil {
		cfg.UpstreamEnabled = f.Upstream.Enabled
		if f.Upstream.BaseURL != "" {
			cfg.UpstreamBase = f.Upstream.BaseURL
		}
	}

	return nil
}

func mysqlDSNFromFile(m mysqlFile) string {
	if m.DSN != "" {
		return m.DSN
	}
	if m.Host == "" && m.User == "" && m.Database == "" {
		return ""
	}
	host := m.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := m.Port
	if port == 0 {
		port = 3306
	}
	user := m.User
	if user == "" {
		user = "atlas"
	}
	db := m.Database
	if db == "" {
		db = "atlas"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		user, m.Password, host, port, db)
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("ATLAS_ADDR"); v != "" {
		cfg.Addr = v
	}
	if v := os.Getenv("ATLAS_PUBLIC_BASE_URL"); v != "" {
		cfg.PublicBaseURL = v
	}
	if v := os.Getenv("ATLAS_JWT_SECRET"); v != "" {
		cfg.JWTSecret = []byte(v)
	}
	if d, ok := durationFromEnv("ATLAS_ACCESS_TTL"); ok {
		cfg.AccessTokenTTL = d
	}
	if d, ok := durationFromEnv("ATLAS_REFRESH_TTL"); ok {
		cfg.RefreshTTL = d
	}
	if d, ok := durationFromEnv("ATLAS_DEVICE_TTL"); ok {
		cfg.DeviceTTL = d
	}
	if d, ok := durationFromEnv("ATLAS_SESSION_TTL"); ok {
		cfg.SessionTTL = d
	}
	if v := os.Getenv("ATLAS_DEFAULT_USER_ID"); v != "" {
		cfg.DefaultUserID = v
	}
	if v := os.Getenv("ATLAS_DEFAULT_EMAIL"); v != "" {
		cfg.DefaultEmail = v
	}
	if v := os.Getenv("ATLAS_BOOTSTRAP_PASSWORD"); v != "" {
		cfg.BootstrapPassword = v
	}
	if v := os.Getenv("ATLAS_MYSQL_DSN"); v != "" {
		cfg.MySQLDSN = v
	}
	if v := os.Getenv("ATLAS_GROK_HOME"); v != "" {
		cfg.GrokHome = v
	}
	if v := os.Getenv("ATLAS_DOWNLOAD_DIR"); v != "" {
		cfg.DownloadDir = v
	}
	if v := os.Getenv("ATLAS_RELEASES_DIR"); v != "" {
		cfg.ReleasesDir = v
	}
	if v := os.Getenv("ATLAS_UPSTREAM_BASE"); v != "" {
		cfg.UpstreamBase = v
	}
	if v := os.Getenv("ATLAS_UPSTREAM"); v != "" {
		cfg.UpstreamEnabled = v != "0"
	}
}

func resolveConfigPath() string {
	if v := os.Getenv("ATLAS_CONFIG"); v != "" {
		return v
	}
	candidates := []string{
		"atlas-server.toml",
		filepath.Join("config", "atlas-server.toml"),
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, "atlas-server.toml"),
			filepath.Join(dir, "config", "atlas-server.toml"),
		)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func resolveDownloadDir(override string) string {
	if override != "" {
		return override
	}
	if v := os.Getenv("ATLAS_DOWNLOAD_DIR"); v != "" {
		return v
	}
	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		cwd = "."
	}
	download := filepath.Join(cwd, "download")
	downloads := filepath.Join(cwd, "downloads")
	if st, err := os.Stat(download); err == nil && st.IsDir() {
		return download
	}
	if st, err := os.Stat(downloads); err == nil && st.IsDir() {
		return downloads
	}
	return download
}

func resolveReleasesDir(override string) string {
	if override != "" {
		return override
	}
	if v := os.Getenv("ATLAS_RELEASES_DIR"); v != "" {
		return v
	}
	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		cwd = "."
	}
	return filepath.Join(cwd, "releases")
}

func durationFromEnv(key string) (time.Duration, bool) {
	v := os.Getenv(key)
	if v == "" {
		return 0, false
	}
	return parseDuration(v)
}

func parseDuration(v string) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second, true
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d, true
	}
	return 0, false
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
