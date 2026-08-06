package upstream

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/atlas-build/atlas-server/internal/config"
	"github.com/atlas-build/atlas-server/internal/grokdata"
)

// Client proxies selected requests to the real cli-chat-proxy using the
// OAuth token stored in ATLAS_GROK_HOME/auth.json.
type Client struct {
	cfg  config.Config
	data *grokdata.Store
	http *http.Client
}

// New constructs an upstream Client.
func New(cfg config.Config, data *grokdata.Store) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyURL := firstEnv("HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"); proxyURL != "" {
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}
	return &Client{
		cfg:  cfg,
		data: data,
		http: &http.Client{Timeout: 120 * time.Second, Transport: transport},
	}
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// Enabled reports whether upstream proxying is turned on.
func (c *Client) Enabled() bool { return c.cfg.UpstreamEnabled && c.cfg.UpstreamBase != "" }

// Get fetches an upstream path (must start with /) and returns body bytes.
func (c *Client) Get(path string) (body []byte, contentType string, status int, err error) {
	token, err := c.data.AuthToken()
	if err != nil {
		return nil, "", 0, err
	}
	u := c.cfg.UpstreamBase + path
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-XAI-Token-Auth", "xai-grok-cli")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "atlas-server-upstream")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", resp.StatusCode, err
	}
	return b, resp.Header.Get("Content-Type"), resp.StatusCode, nil
}

// ProxyStream copies an upstream POST stream to the client response writer.
func (c *Client) ProxyStream(w http.ResponseWriter, r *http.Request, path string) error {
	token, err := c.data.AuthToken()
	if err != nil {
		return err
	}
	u := c.cfg.UpstreamBase + path
	req, err := http.NewRequestWithContext(r.Context(), r.Method, u, r.Body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-XAI-Token-Auth", "xai-grok-cli")
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if ae := r.Header.Get("Accept"); ae != "" {
		req.Header.Set("Accept", ae)
	} else {
		req.Header.Set("Accept", "text/event-stream")
	}
	req.Header.Set("User-Agent", "atlas-server-upstream")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for k, vals := range resp.Header {
		if strings.EqualFold(k, "Content-Length") {
			continue
		}
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	return err
}

// ProxyEnvHint returns a short diagnostic when upstream is disabled/unusable.
func (c *Client) ProxyEnvHint() string {
	if !c.Enabled() {
		return "ATLAS_UPSTREAM=0"
	}
	if _, err := c.data.AuthToken(); err != nil {
		return fmt.Sprintf("missing token in %s (%v)", c.cfg.GrokHome, err)
	}
	return "ok"
}
