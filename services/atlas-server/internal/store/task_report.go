package store

import (
	"encoding/json"
	"path"
	"strings"
	"time"
)

// Artifact is a single file produced by a subagent task, classified by kind.
type Artifact struct {
	Path string `json:"path"`
	Kind string `json:"kind"` // "code" | "doc" | "other"
}

// TaskReportRecord is one per-subagent-task report: what task ran, which agent
// handled it, and what artifacts it produced, attributed to a user.
type TaskReportRecord struct {
	ID              int64
	UserID          string
	Email           string
	TeamID          string
	SubagentID      string
	ParentSessionID string
	ChildSessionID  string
	SubagentType    string
	Model           string
	ModelRouting    string
	Description     string
	Prompt          string
	Status          string
	Success         bool
	DurationMs      uint64
	ToolCalls       uint32
	Turns           uint32
	TokensUsed      uint64
	Artifacts       []Artifact
	ArtifactCount   int
	Cwd             string
	WorktreePath    string
	Error           string
	StartedAt       string
	CompletedAt     string
	ClientIP        string
	ClientVersion   string
	CreatedAt       time.Time
}

// AgentAggregate is a per-agent rollup of task reports.
type AgentAggregate struct {
	SubagentType  string `json:"subagentType"`
	Count         int    `json:"count"`
	ArtifactCount int    `json:"artifactCount"`
	TokensUsed    uint64 `json:"tokensUsed"`
}

// ModelAggregate is a per-model rollup of task reports.
type ModelAggregate struct {
	Model         string `json:"model"`
	Count         int    `json:"count"`
	ArtifactCount int    `json:"artifactCount"`
	TokensUsed    uint64 `json:"tokensUsed"`
}

// TaskReportSummary is a cross-user totals rollup.
type TaskReportSummary struct {
	TotalTasks     int    `json:"totalTasks"`
	SuccessCount   int    `json:"successCount"`
	FailedCount    int    `json:"failedCount"`
	CancelledCount int    `json:"cancelledCount"`
	TotalArtifacts int    `json:"totalArtifacts"`
	TotalTokens    uint64 `json:"totalTokens"`
	UniqueUsers    int    `json:"uniqueUsers"`
	UniqueModels   int    `json:"uniqueModels"`
}

// UserAggregate is a per-user rollup of task reports.
type UserAggregate struct {
	UserID        string `json:"userId"`
	Email         string `json:"email"`
	Count         int    `json:"count"`
	SuccessCount  int    `json:"successCount"`
	ArtifactCount int    `json:"artifactCount"`
	TokensUsed    uint64 `json:"tokensUsed"`
}

// TaskReportStore persists per-subagent-task reports.
//
// fromDay/toDay are inclusive calendar days of created_at in the server local
// timezone (format "2006-01-02"). Empty or "all" means no date filter.
type TaskReportStore interface {
	InsertTaskReport(r TaskReportRecord) (int64, error)
	ListTaskReportsByUser(userID string, limit int, fromDay, toDay string) ([]TaskReportRecord, error)
	AggregateTaskReportsByAgent(userID string, fromDay, toDay string) ([]AgentAggregate, error)
	AggregateTaskReportsByModel(userID string, fromDay, toDay string) ([]ModelAggregate, error)
	AggregateTaskReportsOverall(fromDay, toDay string) (TaskReportSummary, []AgentAggregate, []ModelAggregate, error)
	AggregateTaskReportsByUser(limit int, fromDay, toDay string) ([]UserAggregate, error)
}

// ParseReportRange parses inclusive YYYY-MM-DD bounds and returns normalized
// from/to day strings for SQL DATE(created_at) filtering.
// Empty/"all" for either bound yields ok=false (no filter).
func ParseReportRange(fromDay, toDay string) (from, to string, ok bool, err error) {
	fromDay = strings.TrimSpace(fromDay)
	toDay = strings.TrimSpace(toDay)
	if fromDay == "" || fromDay == "all" || toDay == "" || toDay == "all" {
		return "", "", false, nil
	}
	start, err := time.ParseInLocation("2006-01-02", fromDay, time.Local)
	if err != nil {
		return "", "", false, err
	}
	end, err := time.ParseInLocation("2006-01-02", toDay, time.Local)
	if err != nil {
		return "", "", false, err
	}
	if end.Before(start) {
		start, end = end, start
	}
	return start.Format("2006-01-02"), end.Format("2006-01-02"), true, nil
}

// reportDateWhere appends a calendar-day filter on created_at.
// Uses DATE(created_at) with string days so MySQL session timezone matches
// the wall-clock day, avoiding go-sql-driver time.Time UTC conversion issues.
func reportDateWhere(prefix string, fromDay, toDay string) (clause string, args []any, err error) {
	from, to, filter, err := ParseReportRange(fromDay, toDay)
	if err != nil || !filter {
		return "", nil, err
	}
	return prefix + `DATE(created_at) >= ? AND DATE(created_at) <= ?`, []any{from, to}, nil
}

// ClassifyArtifacts turns raw paths into classified artifacts (code/doc/other).
func ClassifyArtifacts(paths []string) []Artifact {
	out := make([]Artifact, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, Artifact{Path: p, Kind: classifyArtifact(p)})
	}
	return out
}

func classifyArtifact(p string) string {
	lower := strings.ToLower(filepathToSlash(p))
	if strings.Contains(lower, "documents/") || strings.HasPrefix(lower, "docs/") ||
		strings.Contains(lower, "/docs/") {
		return "doc"
	}
	switch strings.ToLower(strings.TrimPrefix(path.Ext(lower), ".")) {
	case "md", "markdown", "txt", "rst", "adoc":
		return "doc"
	case "go", "rs", "ts", "tsx", "js", "jsx", "py", "java", "kt", "c", "cc", "cpp",
		"h", "hpp", "cs", "rb", "php", "swift", "scala", "sh", "sql", "proto",
		"toml", "yaml", "yml", "json", "html", "css", "vue":
		return "code"
	default:
		return "other"
	}
}

func filepathToSlash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// InsertTaskReport stores a single task report keyed by user.
func (m *MySQLStore) InsertTaskReport(r TaskReportRecord) (int64, error) {
	if r.ArtifactCount == 0 {
		r.ArtifactCount = len(r.Artifacts)
	}
	artifactsJSON, err := json.Marshal(r.Artifacts)
	if err != nil {
		artifactsJSON = []byte("[]")
	}
	res, err := m.db.Exec(
		`INSERT INTO task_reports
			(user_id, email, team_id, subagent_id, parent_session_id, child_session_id,
			 subagent_type, model, model_routing, description, prompt, status, success, duration_ms,
			 tool_calls, turns, tokens_used, artifacts, artifact_count, cwd, worktree_path,
			 error, started_at, completed_at, client_ip, client_version)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullIfEmpty(r.UserID),
		nullIfEmpty(r.Email),
		nullIfEmpty(r.TeamID),
		nullIfEmpty(r.SubagentID),
		nullIfEmpty(r.ParentSessionID),
		nullIfEmpty(r.ChildSessionID),
		r.SubagentType,
		nullIfEmpty(r.Model),
		nullIfEmpty(r.ModelRouting),
		nullIfEmpty(r.Description),
		nullIfEmpty(r.Prompt),
		r.Status,
		r.Success,
		r.DurationMs,
		r.ToolCalls,
		r.Turns,
		r.TokensUsed,
		string(artifactsJSON),
		r.ArtifactCount,
		nullIfEmpty(r.Cwd),
		nullIfEmpty(r.WorktreePath),
		nullIfEmpty(r.Error),
		nullIfEmpty(r.StartedAt),
		nullIfEmpty(r.CompletedAt),
		nullIfEmpty(r.ClientIP),
		nullIfEmpty(r.ClientVersion),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListTaskReportsByUser returns recent task reports for a user (newest first).
func (m *MySQLStore) ListTaskReportsByUser(userID string, limit int, fromDay, toDay string) ([]TaskReportRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	dateClause, dateArgs, err := reportDateWhere(" AND ", fromDay, toDay)
	if err != nil {
		return nil, err
	}
	q := `SELECT id, IFNULL(user_id,''), IFNULL(email,''), IFNULL(team_id,''),
		        IFNULL(subagent_id,''), IFNULL(parent_session_id,''), IFNULL(child_session_id,''),
		        subagent_type, IFNULL(model,''), IFNULL(model_routing,''), IFNULL(description,''), IFNULL(prompt,''),
		        status, success, duration_ms, tool_calls, turns, tokens_used,
		        IFNULL(artifacts,'[]'), artifact_count, IFNULL(cwd,''), IFNULL(worktree_path,''),
		        IFNULL(error,''), IFNULL(started_at,''), IFNULL(completed_at,''),
		        IFNULL(client_ip,''), IFNULL(client_version,''), created_at
		 FROM task_reports
		 WHERE user_id = ?` + dateClause + `
		 ORDER BY id DESC LIMIT ?`
	args := append([]any{userID}, dateArgs...)
	args = append(args, limit)

	rows, err := m.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TaskReportRecord
	for rows.Next() {
		var r TaskReportRecord
		var artifactsJSON string
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.Email, &r.TeamID,
			&r.SubagentID, &r.ParentSessionID, &r.ChildSessionID,
			&r.SubagentType, &r.Model, &r.ModelRouting, &r.Description, &r.Prompt,
			&r.Status, &r.Success, &r.DurationMs, &r.ToolCalls, &r.Turns, &r.TokensUsed,
			&artifactsJSON, &r.ArtifactCount, &r.Cwd, &r.WorktreePath,
			&r.Error, &r.StartedAt, &r.CompletedAt,
			&r.ClientIP, &r.ClientVersion, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		if artifactsJSON != "" {
			_ = json.Unmarshal([]byte(artifactsJSON), &r.Artifacts)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AggregateTaskReportsByAgent returns per-agent counts for a user.
func (m *MySQLStore) AggregateTaskReportsByAgent(userID string, fromDay, toDay string) ([]AgentAggregate, error) {
	dateClause, dateArgs, err := reportDateWhere(" AND ", fromDay, toDay)
	if err != nil {
		return nil, err
	}
	q := `SELECT subagent_type, COUNT(*), IFNULL(SUM(artifact_count),0), IFNULL(SUM(tokens_used),0)
		 FROM task_reports
		 WHERE user_id = ?` + dateClause + `
		 GROUP BY subagent_type ORDER BY COUNT(*) DESC`
	args := append([]any{userID}, dateArgs...)

	rows, err := m.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AgentAggregate
	for rows.Next() {
		var a AgentAggregate
		if err := rows.Scan(&a.SubagentType, &a.Count, &a.ArtifactCount, &a.TokensUsed); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AggregateTaskReportsByModel returns per-model counts for a user.
func (m *MySQLStore) AggregateTaskReportsByModel(userID string, fromDay, toDay string) ([]ModelAggregate, error) {
	dateClause, dateArgs, err := reportDateWhere(" AND ", fromDay, toDay)
	if err != nil {
		return nil, err
	}
	q := `SELECT IFNULL(NULLIF(TRIM(model), ''), '(unknown)'),
		        COUNT(*), IFNULL(SUM(artifact_count),0), IFNULL(SUM(tokens_used),0)
		 FROM task_reports
		 WHERE user_id = ?` + dateClause + `
		 GROUP BY IFNULL(NULLIF(TRIM(model), ''), '(unknown)')
		 ORDER BY COUNT(*) DESC`
	args := append([]any{userID}, dateArgs...)

	rows, err := m.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ModelAggregate
	for rows.Next() {
		var a ModelAggregate
		if err := rows.Scan(&a.Model, &a.Count, &a.ArtifactCount, &a.TokensUsed); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AggregateTaskReportsOverall returns cross-user totals and per-agent / per-model rollups.
func (m *MySQLStore) AggregateTaskReportsOverall(fromDay, toDay string) (TaskReportSummary, []AgentAggregate, []ModelAggregate, error) {
	where, args, err := reportDateWhere(" WHERE ", fromDay, toDay)
	if err != nil {
		return TaskReportSummary{}, nil, nil, err
	}

	var s TaskReportSummary
	err = m.db.QueryRow(
		`SELECT
			COUNT(*),
			IFNULL(SUM(success), 0),
			IFNULL(SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END), 0),
			IFNULL(SUM(CASE WHEN success = 0 AND status <> 'cancelled' THEN 1 ELSE 0 END), 0),
			IFNULL(SUM(artifact_count), 0),
			IFNULL(SUM(tokens_used), 0),
			COUNT(DISTINCT user_id),
			COUNT(DISTINCT NULLIF(TRIM(model), ''))
		 FROM task_reports`+where,
		args...,
	).Scan(
		&s.TotalTasks,
		&s.SuccessCount,
		&s.CancelledCount,
		&s.FailedCount,
		&s.TotalArtifacts,
		&s.TotalTokens,
		&s.UniqueUsers,
		&s.UniqueModels,
	)
	if err != nil {
		return TaskReportSummary{}, nil, nil, err
	}

	rows, err := m.db.Query(
		`SELECT subagent_type, COUNT(*), IFNULL(SUM(artifact_count),0), IFNULL(SUM(tokens_used),0)
		 FROM task_reports`+where+`
		 GROUP BY subagent_type
		 ORDER BY COUNT(*) DESC`,
		args...,
	)
	if err != nil {
		return TaskReportSummary{}, nil, nil, err
	}
	var agents []AgentAggregate
	for rows.Next() {
		var a AgentAggregate
		if err := rows.Scan(&a.SubagentType, &a.Count, &a.ArtifactCount, &a.TokensUsed); err != nil {
			rows.Close()
			return TaskReportSummary{}, nil, nil, err
		}
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return TaskReportSummary{}, nil, nil, err
	}
	rows.Close()

	modelRows, err := m.db.Query(
		`SELECT IFNULL(NULLIF(TRIM(model), ''), '(unknown)'),
		        COUNT(*), IFNULL(SUM(artifact_count),0), IFNULL(SUM(tokens_used),0)
		 FROM task_reports`+where+`
		 GROUP BY IFNULL(NULLIF(TRIM(model), ''), '(unknown)')
		 ORDER BY COUNT(*) DESC`,
		args...,
	)
	if err != nil {
		return TaskReportSummary{}, nil, nil, err
	}
	defer modelRows.Close()

	var models []ModelAggregate
	for modelRows.Next() {
		var a ModelAggregate
		if err := modelRows.Scan(&a.Model, &a.Count, &a.ArtifactCount, &a.TokensUsed); err != nil {
			return TaskReportSummary{}, nil, nil, err
		}
		models = append(models, a)
	}
	return s, agents, models, modelRows.Err()
}

// AggregateTaskReportsByUser returns per-user rollups ordered by task count.
func (m *MySQLStore) AggregateTaskReportsByUser(limit int, fromDay, toDay string) ([]UserAggregate, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	where, dateArgs, err := reportDateWhere(" WHERE ", fromDay, toDay)
	if err != nil {
		return nil, err
	}
	q := `SELECT
			IFNULL(user_id, ''),
			IFNULL(MAX(NULLIF(email, '')), ''),
			COUNT(*),
			IFNULL(SUM(success), 0),
			IFNULL(SUM(artifact_count), 0),
			IFNULL(SUM(tokens_used), 0)
		 FROM task_reports` + where + `
		 GROUP BY user_id ORDER BY COUNT(*) DESC LIMIT ?`
	args := append(dateArgs, limit)

	rows, err := m.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserAggregate
	for rows.Next() {
		var u UserAggregate
		if err := rows.Scan(
			&u.UserID, &u.Email, &u.Count, &u.SuccessCount, &u.ArtifactCount, &u.TokensUsed,
		); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

var _ TaskReportStore = (*MySQLStore)(nil)
