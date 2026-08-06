package proxy

import (
	"encoding/json"
	"net/http"

	"github.com/atlas-build/atlas-server/internal/grokdata"
	"github.com/atlas-build/atlas-server/internal/upstream"
)

// Handler stubs / serves optional cli-chat-proxy endpoints from ~/.grok caches.
type Handler struct {
	data *grokdata.Store
	up   *upstream.Client
}

// NewHandler constructs a proxy Handler.
func NewHandler(data *grokdata.Store, up *upstream.Client) *Handler {
	return &Handler{data: data, up: up}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeRaw(w http.ResponseWriter, status int, ctype string, b []byte) {
	if ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func (h *Handler) serveProbeOrUpstream(w http.ResponseWriter, probeName, upstreamPath, fallbackJSON string) {
	if b, ok := h.data.ReadProbeJSON(probeName); ok {
		writeRaw(w, http.StatusOK, "application/json", b)
		return
	}
	if h.up != nil && h.up.Enabled() {
		if body, ctype, status, err := h.up.Get(upstreamPath); err == nil && status >= 200 && status < 300 {
			_ = h.data.SaveProbe(probeName, body)
			if ctype == "" {
				ctype = "application/json"
			}
			writeRaw(w, http.StatusOK, ctype, body)
			return
		}
	}
	writeRaw(w, http.StatusOK, "application/json", []byte(fallbackJSON))
}

// BillingCredits serves real billing shape when available.
func (h *Handler) BillingCredits(w http.ResponseWriter, r *http.Request) {
	h.serveProbeOrUpstream(w, "probe_billing_format=credits.json", "/billing?format=credits",
		`{"config":{"onDemandCap":{"val":0},"onDemandUsed":{"val":0},"prepaidBalance":{"val":999999}}}`)
}

// McpConfigs must use mcp_servers (not configs).
func (h *Handler) McpConfigs(w http.ResponseWriter, r *http.Request) {
	h.serveProbeOrUpstream(w, "probe_mcp_configs.json", "/mcp/configs", `{"mcp_servers":[]}`)
}

// FeedbackConfig returns captured feedback config.
func (h *Handler) FeedbackConfig(w http.ResponseWriter, r *http.Request) {
	h.serveProbeOrUpstream(w, "probe_feedback_config.json", "/feedback/config", `{"enabled":false}`)
}

// BundleArchive serves gzip tar from probe cache, or packs ~/.grok/bundled (+ user skills).
func (h *Handler) BundleArchive(w http.ResponseWriter, r *http.Request) {
	if b, ok := h.data.ReadProbeFile("probe_bundle_archive"); ok {
		writeRaw(w, http.StatusOK, "application/gzip", b)
		return
	}
	if h.up != nil && h.up.Enabled() {
		if body, ctype, status, err := h.up.Get("/bundle/archive"); err == nil && status >= 200 && status < 300 {
			_ = h.data.SaveProbe("probe_bundle_archive", body)
			if ctype == "" {
				ctype = "application/gzip"
			}
			writeRaw(w, http.StatusOK, ctype, body)
			return
		}
	}
	arch, err := h.data.BuildBundleArchive()
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "bundle_not_available", "detail": err.Error()})
		return
	}
	writeRaw(w, http.StatusOK, "application/gzip", arch)
}

// SubagentsBundle serves legacy JSON bundle from probe, filesystem, or upstream.
func (h *Handler) SubagentsBundle(w http.ResponseWriter, r *http.Request) {
	if b, ok := h.data.ReadProbeJSON("probe_subagents_bundle.json"); ok {
		writeRaw(w, http.StatusOK, "application/json", b)
		return
	}
	if body, err := h.data.BuildSubagentBundleJSON(); err == nil && len(body) > 2 {
		writeRaw(w, http.StatusOK, "application/json", body)
		return
	}
	if h.up != nil && h.up.Enabled() {
		if body, ctype, status, err := h.up.Get("/subagents/bundle"); err == nil && status >= 200 && status < 300 {
			_ = h.data.SaveProbe("probe_subagents_bundle.json", body)
			if ctype == "" {
				ctype = "application/json"
			}
			writeRaw(w, http.StatusOK, ctype, body)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":  "empty",
		"personas": map[string]string{},
		"roles":    map[string]string{},
		"agents":   map[string]string{},
		"skills":   map[string]string{},
	})
}
