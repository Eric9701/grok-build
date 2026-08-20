package server

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/atlas-build/atlas-server/internal/account"
	"github.com/atlas-build/atlas-server/internal/admin"
	"github.com/atlas-build/atlas-server/internal/auth"
	"github.com/atlas-build/atlas-server/internal/config"
	"github.com/atlas-build/atlas-server/internal/crypto"
	"github.com/atlas-build/atlas-server/internal/grokdata"
	"github.com/atlas-build/atlas-server/internal/inference"
	"github.com/atlas-build/atlas-server/internal/proxy"
	"github.com/atlas-build/atlas-server/internal/releases"
	"github.com/atlas-build/atlas-server/internal/settings"
	"github.com/atlas-build/atlas-server/internal/skills"
	"github.com/atlas-build/atlas-server/internal/store"
	"github.com/atlas-build/atlas-server/internal/telemetry"
	"github.com/atlas-build/atlas-server/internal/upstream"
	"github.com/atlas-build/atlas-server/internal/user"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Pages holds parsed HTML templates for web UI.
type Pages struct {
	Device  *template.Template
	Login   *template.Template
	Account *template.Template
}

// New builds the HTTP handler tree for atlas-server.
// All application routes are mounted under config.PathPrefix (/atlas).
func New(cfg config.Config, st store.Store, users store.UserStore, pages Pages) http.Handler {
	data := grokdata.New(cfg.GrokHome, cfg.DownloadDir)
	up := upstream.New(cfg, data)
	log.Printf("download dir: %s | releases dir: %s | grok home: %s (upstream=%v hint=%s)",
		cfg.DownloadDir, cfg.ReleasesDir, cfg.GrokHome, up.Enabled(), up.ProxyEnvHint())

	releases.EnsureDir(cfg.ReleasesDir)
	releasesH := releases.NewHandler(cfg.ReleasesDir)

	accountH := account.NewHandler(cfg, users, pages.Login)
	authH := auth.NewHandler(cfg, st, users, accountH, pages.Device)
	userH := user.NewHandler(cfg, authH, users, data, up)
	settingsH := settings.NewHandler(data, up)
	skillsH := skills.NewHandler()
	var traceStore store.TraceStore
	if ts, ok := users.(store.TraceStore); ok {
		traceStore = ts
	}
	var reportStore store.TaskReportStore
	if rs, ok := users.(store.TaskReportStore); ok {
		reportStore = rs
	}
	var signalsStore store.SessionSignalsStore
	if ss, ok := users.(store.SessionSignalsStore); ok {
		signalsStore = ss
	}
	telemetryH := telemetry.NewHandler(authH, traceStore, reportStore, signalsStore, users)
	adminH := admin.NewHandler()
	var managedStore store.ManagedModelStore
	if ms, ok := users.(store.ManagedModelStore); ok {
		managedStore = ms
	}
	usersAdminH := admin.NewUsersHandler(users)
	modelsAdminH := admin.NewModelsHandler(authH, managedStore, crypto.ResolveModelSecret())
	inferenceH := inference.NewHandler(data, up).WithManagedModels(authH, managedStore, users)
	proxyH := proxy.NewHandler(data, up)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	healthz := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
	r.Get("/healthz", healthz)

	// Convenience redirects when PathPrefix (/atlas) is omitted.
	r.Get("/admin/task-reports", redirectToPathPrefix)
	r.Get("/admin/task-reports/", redirectToPathPrefix)
	r.Get("/admin/models", redirectToPathPrefix)
	r.Get("/admin/models/", redirectToPathPrefix)
	r.Get("/admin/groups", redirectToPathPrefix)
	r.Get("/admin/groups/", redirectToPathPrefix)
	r.Get("/admin/users", redirectToPathPrefix)
	r.Get("/admin/users/", redirectToPathPrefix)

	r.Route(config.PathPrefix, func(r chi.Router) {
		r.Get("/healthz", healthz)

		// CLI update artifacts (channel pointers + binaries).
		r.Get("/cli/*", releasesH.ServeHTTP)
		r.Head("/cli/*", releasesH.ServeHTTP)

		// Web account login (required before device-code approval).
		r.Get("/login", accountH.LoginPage)
		r.Post("/login", accountH.LoginPage)
		r.Get("/register", accountH.RegisterPage)
		r.Post("/register", accountH.RegisterPage)
		r.Get("/logout", accountH.Logout)
		r.Get("/account", func(w http.ResponseWriter, r *http.Request) {
			u, ok := accountH.CurrentUser(r)
			if !ok {
				http.Redirect(w, r, config.Path("/login?next=/atlas/account"), http.StatusFound)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_ = pages.Account.Execute(w, map[string]any{
				"Email":       u.Email,
				"FirstName":   u.FirstName,
				"LastName":    u.LastName,
				"UserID":      u.UserID,
				"MachineCode": u.MachineCode,
			})
		})
		r.Post("/api/auth/login", accountH.LoginAPI)
		r.Get("/api/auth/me", accountH.Me)

		r.Get("/.well-known/openid-configuration", authH.Discovery)
		// Browser authorize URL → machine-code (device) login page.
		r.Get("/authorize", authH.Authorize)
		r.Post("/oauth2/device/code", authH.RequestDeviceCode)
		r.Get("/oauth2/device", authH.DevicePage)
		r.Post("/oauth2/device", authH.DevicePage)
		r.Post("/oauth2/token", authH.Token)

		r.Route("/v1", func(r chi.Router) {
			r.Get("/user", userH.GetUser)
			r.Get("/settings", settingsH.GetSettings)
			r.Get("/models", inferenceH.ListModels)
			r.Post("/responses", inferenceH.Responses)
			r.Get("/skills", skillsH.List)
			r.Post("/events", telemetryH.IngestEvents)
			r.Post("/traces", telemetryH.Traces)
			r.Post("/task-reports", telemetryH.TaskReports)
			r.Post("/sessions/{sessionId}/signals", telemetryH.UpdateSessionSignals)

			r.Get("/billing", proxyH.BillingCredits)
			r.Get("/mcp/configs", proxyH.McpConfigs)
			r.Get("/feedback/config", proxyH.FeedbackConfig)
			r.Get("/bundle/archive", proxyH.BundleArchive)
			r.Get("/subagents/bundle", proxyH.SubagentsBundle)
		})

		r.Route("/admin/api", func(r chi.Router) {
			r.Get("/status", adminH.Status)
			r.Get("/traces", telemetryH.ListTraces)
			r.Get("/task-reports", telemetryH.ListTaskReports)
			r.Get("/session-signals", telemetryH.ListSessionSignals)

			r.Get("/managed-models", modelsAdminH.ListManagedModels)
			r.Post("/managed-models", modelsAdminH.UpsertManagedModel)
			r.Put("/managed-models/{id}", modelsAdminH.UpsertManagedModel)
			r.Delete("/managed-models/{id}", modelsAdminH.DeleteManagedModel)
			r.Get("/users", modelsAdminH.ListUsers)
			r.Get("/users/{userId}/models", modelsAdminH.GetUserModels)
			r.Put("/users/{userId}/models", modelsAdminH.SetUserModels)
			r.Get("/users/{userId}/effective-models", modelsAdminH.GetEffectiveModels)

			r.Get("/groups", modelsAdminH.ListUserGroups)
			r.Post("/groups", modelsAdminH.CreateUserGroup)
			r.Put("/groups/{groupId}", modelsAdminH.UpdateUserGroup)
			r.Delete("/groups/{groupId}", modelsAdminH.DeleteUserGroup)
			r.Get("/groups/{groupId}/members", modelsAdminH.GetGroupMembers)
			r.Put("/groups/{groupId}/members", modelsAdminH.SetGroupMembers)
			r.Get("/groups/{groupId}/models", modelsAdminH.GetGroupModels)
			r.Put("/groups/{groupId}/models", modelsAdminH.SetGroupModels)

			r.Post("/crypto/encrypt", modelsAdminH.Encrypt)
			r.Post("/crypto/decrypt", modelsAdminH.Decrypt)

			r.Post("/users", usersAdminH.CreateUser)
			r.Get("/users/all", usersAdminH.ListAllUsers)
		})

		r.Get("/admin/task-reports", serveTaskReportsPage)
		r.Get("/admin/task-reports/", serveTaskReportsPage)
		r.Get("/admin/models", serveModelsPage)
		r.Get("/admin/models/", serveModelsPage)
		r.Get("/admin/groups", serveGroupsPage)
		r.Get("/admin/groups/", serveGroupsPage)
		r.Get("/admin/users", serveUsersPage)
		r.Get("/admin/users/", serveUsersPage)
	})

	return r
}

func serveTaskReportsPage(w http.ResponseWriter, r *http.Request) {
	path := resolveWebPath("web/admin/task-reports/index.html")
	http.ServeFile(w, r, path)
}

func serveModelsPage(w http.ResponseWriter, r *http.Request) {
	path := resolveWebPath("web/admin/models/index.html")
	http.ServeFile(w, r, path)
}

func serveUsersPage(w http.ResponseWriter, r *http.Request) {
	path := resolveWebPath("web/admin/users/index.html")
	http.ServeFile(w, r, path)
}

func serveGroupsPage(w http.ResponseWriter, r *http.Request) {
	path := resolveWebPath("web/admin/groups/index.html")
	http.ServeFile(w, r, path)
}

func redirectToPathPrefix(w http.ResponseWriter, r *http.Request) {
	target := config.PathPrefix + r.URL.Path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func resolveWebPath(rel string) string {
	candidates := []string{rel, filepath.Join("services", "atlas-server", rel)}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates,
			filepath.Join(dir, rel),
			filepath.Join(dir, "..", rel),
		)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return rel
}
