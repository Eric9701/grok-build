package telemetry

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/atlas-build/atlas-server/internal/store"
)

// resolveReportRange returns inclusive YYYY-MM-DD bounds for admin queries.
//
//	from / to omitted → today (server local) for both
//	legacy date=YYYY-MM-DD → from=to=date
//	from=all / to=all / date=all → no date filter
func resolveReportRange(fromRaw, toRaw, dateRaw string) (from, to string, err error) {
	fromRaw = strings.TrimSpace(fromRaw)
	toRaw = strings.TrimSpace(toRaw)
	dateRaw = strings.TrimSpace(dateRaw)
	if fromRaw == "all" || toRaw == "all" || dateRaw == "all" {
		return "all", "all", nil
	}
	today := time.Now().Format("2006-01-02")
	if fromRaw == "" && toRaw == "" && dateRaw != "" {
		fromRaw, toRaw = dateRaw, dateRaw
	}
	if fromRaw == "" {
		fromRaw = today
	}
	if toRaw == "" {
		toRaw = today
	}
	from, to, filter, err := store.ParseReportRange(fromRaw, toRaw)
	if err != nil {
		return "", "", err
	}
	if !filter {
		return "all", "all", nil
	}
	return from, to, nil
}

// taskReportPayload is the JSON body the CLI POSTs to /v1/task-reports.
// Field names mirror the CLI `TaskReport` (camelCase).
type taskReportPayload struct {
	SubagentID      string   `json:"subagentId"`
	ParentSessionID string   `json:"parentSessionId"`
	ChildSessionID  string   `json:"childSessionId"`
	SubagentType    string   `json:"subagentType"`
	Model           string   `json:"model"`
	Description     string   `json:"description"`
	Prompt          string   `json:"prompt"`
	Status          string   `json:"status"`
	Success         bool     `json:"success"`
	DurationMs      uint64   `json:"durationMs"`
	ToolCalls       uint32   `json:"toolCalls"`
	Turns           uint32   `json:"turns"`
	TokensUsed      uint64   `json:"tokensUsed"`
	Artifacts       []string `json:"artifacts"`
	ArtifactCount   int      `json:"artifactCount"`
	Cwd             string   `json:"cwd"`
	WorktreePath    string   `json:"worktreePath"`
	Error           string   `json:"error"`
	StartedAt       string   `json:"startedAt"`
	CompletedAt     string   `json:"completedAt"`
	UserID          string   `json:"userId"`
	Email           string   `json:"email"`
	ClientVersion   string   `json:"clientVersion"`
}

// TaskReports stores a per-subagent-task report attributed to the user.
// POST /v1/task-reports
func (h *Handler) TaskReports(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	if len(body) > maxBodyBytes {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return
	}

	var p taskReportPayload
	if err := json.Unmarshal(body, &p); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	userID := strings.TrimSpace(p.UserID)
	email := strings.TrimSpace(p.Email)
	if userID == "" {
		userID = "anonymous"
	}
	teamID := strings.TrimSpace(r.Header.Get("x-teamid"))

	if h.reports == nil {
		log.Printf("telemetry: drop task report user=%s agent=%s (no store)", userID, p.SubagentType)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	rec := store.TaskReportRecord{
		UserID:          userID,
		Email:           email,
		TeamID:          teamID,
		SubagentID:      p.SubagentID,
		ParentSessionID: p.ParentSessionID,
		ChildSessionID:  p.ChildSessionID,
		SubagentType:    p.SubagentType,
		Model:           p.Model,
		Description:     p.Description,
		Prompt:          p.Prompt,
		Status:          p.Status,
		Success:         p.Success,
		DurationMs:      p.DurationMs,
		ToolCalls:       p.ToolCalls,
		Turns:           p.Turns,
		TokensUsed:      p.TokensUsed,
		Artifacts:       store.ClassifyArtifacts(p.Artifacts),
		ArtifactCount:   p.ArtifactCount,
		Cwd:             p.Cwd,
		WorktreePath:    p.WorktreePath,
		Error:           p.Error,
		StartedAt:       p.StartedAt,
		CompletedAt:     p.CompletedAt,
		ClientIP:        clientIP(r),
		ClientVersion:   strings.TrimSpace(p.ClientVersion),
	}
	if rec.ArtifactCount == 0 {
		rec.ArtifactCount = len(rec.Artifacts)
	}

	id, err := h.reports.InsertTaskReport(rec)
	if err != nil {
		log.Printf("telemetry: insert task report failed user=%s: %v", userID, err)
		http.Error(w, "persist failed", http.StatusInternalServerError)
		return
	}
	log.Printf("telemetry: stored task report id=%d user=%s agent=%s artifacts=%d",
		id, userID, p.SubagentType, rec.ArtifactCount)
	w.WriteHeader(http.StatusAccepted)
}

// ListTaskReports returns task report analytics (admin).
//
//	GET /admin/api/task-reports?from=&to=                     → overall {summary, agents, models, users}
//	GET /admin/api/task-reports?user_id=&aggregate=1&from=&to= → {agents, models}
//	GET /admin/api/task-reports?user_id=&limit=50&from=&to=
//
// from/to are inclusive YYYY-MM-DD (server local). Both default to today.
// Legacy date=YYYY-MM-DD sets from=to. from=all / to=all / date=all disables the filter.
func (h *Handler) ListTaskReports(w http.ResponseWriter, r *http.Request) {
	if h.reports == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "task report store unavailable"})
		return
	}
	q := r.URL.Query()
	from, to, err := resolveReportRange(q.Get("from"), q.Get("to"), q.Get("date"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date range (want YYYY-MM-DD, or all)"})
		return
	}
	userID := strings.TrimSpace(q.Get("user_id"))
	if userID == "" {
		if email := strings.TrimSpace(q.Get("email")); email != "" && h.users != nil {
			if u, err := h.users.GetUserByEmail(email); err == nil && u != nil {
				userID = u.UserID
			}
		}
	}

	// Overall: no user filter.
	if userID == "" {
		limit, _ := strconv.Atoi(q.Get("limit"))
		summary, agents, models, err := h.reports.AggregateTaskReportsOverall(from, to)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		users, err := h.reports.AggregateTaskReportsByUser(limit, from, to)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if agents == nil {
			agents = []store.AgentAggregate{}
		}
		if models == nil {
			models = []store.ModelAggregate{}
		}
		if users == nil {
			users = []store.UserAggregate{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"from":    from,
			"to":      to,
			"summary": summary,
			"agents":  agents,
			"models":  models,
			"users":   users,
		})
		return
	}

	if agg := strings.TrimSpace(q.Get("aggregate")); agg == "1" || agg == "true" {
		agents, err := h.reports.AggregateTaskReportsByAgent(userID, from, to)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		models, err := h.reports.AggregateTaskReportsByModel(userID, from, to)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if agents == nil {
			agents = []store.AgentAggregate{}
		}
		if models == nil {
			models = []store.ModelAggregate{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"from":   from,
			"to":     to,
			"userId": userID,
			"agents": agents,
			"models": models,
		})
		return
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	rows, err := h.reports.ListTaskReportsByUser(userID, limit, from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type item struct {
		ID              int64            `json:"id"`
		UserID          string           `json:"userId"`
		Email           string           `json:"email,omitempty"`
		TeamID          string           `json:"teamId,omitempty"`
		SubagentID      string           `json:"subagentId,omitempty"`
		ParentSessionID string           `json:"parentSessionId,omitempty"`
		ChildSessionID  string           `json:"childSessionId,omitempty"`
		SubagentType    string           `json:"subagentType"`
		Model           string           `json:"model,omitempty"`
		Description     string           `json:"description,omitempty"`
		Prompt          string           `json:"prompt,omitempty"`
		Status          string           `json:"status"`
		Success         bool             `json:"success"`
		DurationMs      uint64           `json:"durationMs"`
		ToolCalls       uint32           `json:"toolCalls"`
		Turns           uint32           `json:"turns"`
		TokensUsed      uint64           `json:"tokensUsed"`
		Artifacts       []store.Artifact `json:"artifacts"`
		ArtifactCount   int              `json:"artifactCount"`
		Cwd             string           `json:"cwd,omitempty"`
		WorktreePath    string           `json:"worktreePath,omitempty"`
		Error           string           `json:"error,omitempty"`
		StartedAt       string           `json:"startedAt,omitempty"`
		CompletedAt     string           `json:"completedAt,omitempty"`
		ClientIP        string           `json:"clientIp,omitempty"`
		ClientVersion   string           `json:"clientVersion,omitempty"`
		CreatedAt       string           `json:"createdAt"`
	}
	out := make([]item, 0, len(rows))
	for _, row := range rows {
		artifacts := row.Artifacts
		if artifacts == nil {
			artifacts = []store.Artifact{}
		}
		out = append(out, item{
			ID:              row.ID,
			UserID:          row.UserID,
			Email:           row.Email,
			TeamID:          row.TeamID,
			SubagentID:      row.SubagentID,
			ParentSessionID: row.ParentSessionID,
			ChildSessionID:  row.ChildSessionID,
			SubagentType:    row.SubagentType,
			Model:           row.Model,
			Description:     row.Description,
			Prompt:          row.Prompt,
			Status:          row.Status,
			Success:         row.Success,
			DurationMs:      row.DurationMs,
			ToolCalls:       row.ToolCalls,
			Turns:           row.Turns,
			TokensUsed:      row.TokensUsed,
			Artifacts:       artifacts,
			ArtifactCount:   row.ArtifactCount,
			Cwd:             row.Cwd,
			WorktreePath:    row.WorktreePath,
			Error:           row.Error,
			StartedAt:       row.StartedAt,
			CompletedAt:     row.CompletedAt,
			ClientIP:        row.ClientIP,
			ClientVersion:   row.ClientVersion,
			CreatedAt:       row.CreatedAt.UTC().Format(timeRFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"from":    from,
		"to":      to,
		"userId":  userID,
		"count":   len(out),
		"reports": out,
	})
}
