package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/atlas-build/atlas-server/internal/store"
)

// fakeReportStore is an in-memory TaskReportStore for handler tests.
type fakeReportStore struct {
	inserted []store.TaskReportRecord
}

func (f *fakeReportStore) InsertTaskReport(r store.TaskReportRecord) (int64, error) {
	r.ID = int64(len(f.inserted) + 1)
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now()
	}
	f.inserted = append(f.inserted, r)
	return r.ID, nil
}

func (f *fakeReportStore) matchRange(r store.TaskReportRecord, fromDay, toDay string) bool {
	from, to, filter, err := store.ParseReportRange(fromDay, toDay)
	if err != nil || !filter {
		return err == nil
	}
	day := r.CreatedAt.In(time.Local).Format("2006-01-02")
	return day >= from && day <= to
}

func (f *fakeReportStore) ListTaskReportsByUser(userID string, limit int, fromDay, toDay string) ([]store.TaskReportRecord, error) {
	var out []store.TaskReportRecord
	for i := len(f.inserted) - 1; i >= 0; i-- {
		r := f.inserted[i]
		if r.UserID == userID && f.matchRange(r, fromDay, toDay) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeReportStore) AggregateTaskReportsByAgent(userID string, fromDay, toDay string) ([]store.AgentAggregate, error) {
	counts := map[string]*store.AgentAggregate{}
	for _, r := range f.inserted {
		if r.UserID != userID || !f.matchRange(r, fromDay, toDay) {
			continue
		}
		a := counts[r.SubagentType]
		if a == nil {
			a = &store.AgentAggregate{SubagentType: r.SubagentType}
			counts[r.SubagentType] = a
		}
		a.Count++
		a.ArtifactCount += r.ArtifactCount
		a.TokensUsed += r.TokensUsed
	}
	var out []store.AgentAggregate
	for _, a := range counts {
		out = append(out, *a)
	}
	return out, nil
}

func (f *fakeReportStore) AggregateTaskReportsByModel(userID string, fromDay, toDay string) ([]store.ModelAggregate, error) {
	counts := map[string]*store.ModelAggregate{}
	for _, r := range f.inserted {
		if r.UserID != userID || !f.matchRange(r, fromDay, toDay) {
			continue
		}
		key := strings.TrimSpace(r.Model)
		if key == "" {
			key = "(unknown)"
		}
		a := counts[key]
		if a == nil {
			a = &store.ModelAggregate{Model: key}
			counts[key] = a
		}
		a.Count++
		a.ArtifactCount += r.ArtifactCount
		a.TokensUsed += r.TokensUsed
	}
	var out []store.ModelAggregate
	for _, a := range counts {
		out = append(out, *a)
	}
	return out, nil
}

func (f *fakeReportStore) AggregateTaskReportsOverall(fromDay, toDay string) (store.TaskReportSummary, []store.AgentAggregate, []store.ModelAggregate, error) {
	var s store.TaskReportSummary
	users := map[string]struct{}{}
	modelsSeen := map[string]struct{}{}
	agents := map[string]*store.AgentAggregate{}
	models := map[string]*store.ModelAggregate{}
	for _, r := range f.inserted {
		if !f.matchRange(r, fromDay, toDay) {
			continue
		}
		s.TotalTasks++
		s.TotalArtifacts += r.ArtifactCount
		s.TotalTokens += r.TokensUsed
		if r.Success {
			s.SuccessCount++
		} else if r.Status == "cancelled" {
			s.CancelledCount++
		} else {
			s.FailedCount++
		}
		if r.UserID != "" {
			users[r.UserID] = struct{}{}
		}
		if m := strings.TrimSpace(r.Model); m != "" {
			modelsSeen[m] = struct{}{}
		}
		a := agents[r.SubagentType]
		if a == nil {
			a = &store.AgentAggregate{SubagentType: r.SubagentType}
			agents[r.SubagentType] = a
		}
		a.Count++
		a.ArtifactCount += r.ArtifactCount
		a.TokensUsed += r.TokensUsed

		modelKey := strings.TrimSpace(r.Model)
		if modelKey == "" {
			modelKey = "(unknown)"
		}
		ma := models[modelKey]
		if ma == nil {
			ma = &store.ModelAggregate{Model: modelKey}
			models[modelKey] = ma
		}
		ma.Count++
		ma.ArtifactCount += r.ArtifactCount
		ma.TokensUsed += r.TokensUsed
	}
	s.UniqueUsers = len(users)
	s.UniqueModels = len(modelsSeen)
	var agentOut []store.AgentAggregate
	for _, a := range agents {
		agentOut = append(agentOut, *a)
	}
	var modelOut []store.ModelAggregate
	for _, a := range models {
		modelOut = append(modelOut, *a)
	}
	return s, agentOut, modelOut, nil
}

func (f *fakeReportStore) AggregateTaskReportsByUser(limit int, fromDay, toDay string) ([]store.UserAggregate, error) {
	if limit <= 0 {
		limit = 50
	}
	byUser := map[string]*store.UserAggregate{}
	for _, r := range f.inserted {
		if !f.matchRange(r, fromDay, toDay) {
			continue
		}
		u := byUser[r.UserID]
		if u == nil {
			u = &store.UserAggregate{UserID: r.UserID}
			byUser[r.UserID] = u
		}
		if u.Email == "" && r.Email != "" {
			u.Email = r.Email
		}
		u.Count++
		if r.Success {
			u.SuccessCount++
		}
		u.ArtifactCount += r.ArtifactCount
		u.TokensUsed += r.TokensUsed
	}
	var out []store.UserAggregate
	for _, u := range byUser {
		out = append(out, *u)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func TestTaskReportsRoundTrip(t *testing.T) {
	reports := &fakeReportStore{}
	h := NewHandler(nil, nil, reports, nil, nil)

	body := `{
		"subagentId":"sa-1",
		"parentSessionId":"parent-1",
		"childSessionId":"child-1",
		"subagentType":"it-solution-architect",
		"model":"grok-4.5",
		"description":"design the payments module",
		"prompt":"do the thing",
		"status":"completed",
		"success":true,
		"durationMs":1234,
		"toolCalls":7,
		"turns":3,
		"tokensUsed":4096,
		"artifacts":["documents/design.md","services/pay/main.go","assets/logo.bin"],
		"artifactCount":3,
		"cwd":"/repo",
		"startedAt":"2026-07-22T10:00:00Z",
		"completedAt":"2026-07-22T10:05:00Z"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/task-reports", strings.NewReader(body))
	req.Header.Set("x-userid", "atlas-test-user")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.TaskReports(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST status = %d, want 202; body=%s", rec.Code, rec.Body.String())
	}
	if len(reports.inserted) != 1 {
		t.Fatalf("inserted = %d, want 1", len(reports.inserted))
	}
	got := reports.inserted[0]
	if got.UserID != "atlas-test-user" {
		t.Errorf("UserID = %q, want atlas-test-user", got.UserID)
	}
	if got.ArtifactCount != 3 || len(got.Artifacts) != 3 {
		t.Fatalf("artifact count = %d / %d, want 3", got.ArtifactCount, len(got.Artifacts))
	}
	wantKind := map[string]string{
		"documents/design.md":  "doc",
		"services/pay/main.go": "code",
		"assets/logo.bin":      "other",
	}
	for _, a := range got.Artifacts {
		if wantKind[a.Path] != a.Kind {
			t.Errorf("artifact %q kind = %q, want %q", a.Path, a.Kind, wantKind[a.Path])
		}
	}

	today := time.Now().Format("2006-01-02")
	listReq := httptest.NewRequest(http.MethodGet, "/admin/api/task-reports?user_id=atlas-test-user", nil)
	listRec := httptest.NewRecorder()
	h.ListTaskReports(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body=%s", listRec.Code, listRec.Body.String())
	}
	var listResp struct {
		From    string `json:"from"`
		To      string `json:"to"`
		Count   int    `json:"count"`
		Reports []struct {
			SubagentType  string `json:"subagentType"`
			ArtifactCount int    `json:"artifactCount"`
		} `json:"reports"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list resp: %v", err)
	}
	if listResp.From != today || listResp.To != today {
		t.Fatalf("range = %s..%s, want today..today", listResp.From, listResp.To)
	}
	if listResp.Count != 1 || len(listResp.Reports) != 1 {
		t.Fatalf("list count = %d, want 1", listResp.Count)
	}

	aggReq := httptest.NewRequest(http.MethodGet, "/admin/api/task-reports?user_id=atlas-test-user&aggregate=1", nil)
	aggRec := httptest.NewRecorder()
	h.ListTaskReports(aggRec, aggReq)
	if aggRec.Code != http.StatusOK {
		t.Fatalf("aggregate status = %d, want 200", aggRec.Code)
	}
	var aggResp struct {
		Agents []store.AgentAggregate `json:"agents"`
	}
	if err := json.Unmarshal(aggRec.Body.Bytes(), &aggResp); err != nil {
		t.Fatalf("decode aggregate resp: %v", err)
	}
	if len(aggResp.Agents) != 1 || aggResp.Agents[0].Count != 1 || aggResp.Agents[0].ArtifactCount != 3 {
		t.Fatalf("aggregate = %+v", aggResp.Agents)
	}
}

func TestTaskReportsNoStore(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/task-reports", strings.NewReader(`{"subagentType":"x"}`))
	req.Header.Set("x-userid", "u1")
	rec := httptest.NewRecorder()
	h.TaskReports(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (drop)", rec.Code)
	}
}

func TestClassifyArtifacts(t *testing.T) {
	got := store.ClassifyArtifacts([]string{
		"docs/readme.md",
		"src\\lib.rs",
		"data.csv",
		"  ",
	})
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (blank dropped)", len(got))
	}
}

func TestTaskReportsOverall(t *testing.T) {
	reports := &fakeReportStore{}
	h := NewHandler(nil, nil, reports, nil, nil)

	seed := func(userID, email, agent, status string, success bool, artifacts int) {
		body := `{
			"subagentType":"` + agent + `",
			"status":"` + status + `",
			"success":` + boolJSON(success) + `,
			"tokensUsed":100,
			"artifacts":` + artifactJSON(artifacts) + `,
			"artifactCount":` + itoa(artifacts) + `
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1/task-reports", strings.NewReader(body))
		req.Header.Set("x-userid", userID)
		rec := httptest.NewRecorder()
		h.TaskReports(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("seed POST status = %d", rec.Code)
		}
		if n := len(reports.inserted); n > 0 && email != "" {
			reports.inserted[n-1].Email = email
		}
	}
	seed("u1", "a@atlas.local", "explore", "completed", true, 2)
	seed("u1", "a@atlas.local", "explore", "completed", true, 1)
	seed("u2", "b@atlas.local", "coder", "error", false, 0)

	today := time.Now().Format("2006-01-02")
	req := httptest.NewRequest(http.MethodGet, "/admin/api/task-reports", nil)
	rec := httptest.NewRecorder()
	h.ListTaskReports(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("overall status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		From    string                  `json:"from"`
		To      string                  `json:"to"`
		Summary store.TaskReportSummary `json:"summary"`
		Agents  []store.AgentAggregate  `json:"agents"`
		Users   []store.UserAggregate   `json:"users"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.From != today || resp.To != today {
		t.Fatalf("range = %s..%s, want today", resp.From, resp.To)
	}
	if resp.Summary.TotalTasks != 3 || resp.Summary.SuccessCount != 2 || resp.Summary.FailedCount != 1 {
		t.Fatalf("summary = %+v", resp.Summary)
	}
	if len(resp.Users) != 2 || len(resp.Agents) != 2 {
		t.Fatalf("users=%d agents=%d", len(resp.Users), len(resp.Agents))
	}
}

func TestTaskReportsDateRangeFilter(t *testing.T) {
	reports := &fakeReportStore{}
	h := NewHandler(nil, nil, reports, nil, nil)

	today := time.Now()
	yesterday := today.AddDate(0, 0, -1)
	twoDaysAgo := today.AddDate(0, 0, -2)
	reports.inserted = []store.TaskReportRecord{
		{UserID: "u1", Email: "a@x", SubagentType: "explore", Status: "completed", Success: true, ArtifactCount: 1, CreatedAt: today},
		{UserID: "u1", Email: "a@x", SubagentType: "coder", Status: "completed", Success: true, ArtifactCount: 2, CreatedAt: yesterday},
		{UserID: "u2", Email: "b@x", SubagentType: "explore", Status: "error", Success: false, CreatedAt: twoDaysAgo},
	}

	// Default (= today..today): only today.
	req := httptest.NewRequest(http.MethodGet, "/admin/api/task-reports", nil)
	rec := httptest.NewRecorder()
	h.ListTaskReports(rec, req)
	var todayResp struct {
		From    string                  `json:"from"`
		To      string                  `json:"to"`
		Summary store.TaskReportSummary `json:"summary"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &todayResp)
	if todayResp.From != today.Format("2006-01-02") || todayResp.To != today.Format("2006-01-02") {
		t.Fatalf("default range = %s..%s", todayResp.From, todayResp.To)
	}
	if todayResp.Summary.TotalTasks != 1 {
		t.Fatalf("today summary = %+v", todayResp.Summary)
	}

	// Explicit inclusive range covering yesterday + today.
	from := yesterday.Format("2006-01-02")
	to := today.Format("2006-01-02")
	rangeReq := httptest.NewRequest(http.MethodGet, "/admin/api/task-reports?from="+from+"&to="+to, nil)
	rangeRec := httptest.NewRecorder()
	h.ListTaskReports(rangeRec, rangeReq)
	var rangeResp struct {
		From    string                  `json:"from"`
		To      string                  `json:"to"`
		Summary store.TaskReportSummary `json:"summary"`
	}
	_ = json.Unmarshal(rangeRec.Body.Bytes(), &rangeResp)
	if rangeResp.From != from || rangeResp.To != to {
		t.Fatalf("range echo = %s..%s", rangeResp.From, rangeResp.To)
	}
	if rangeResp.Summary.TotalTasks != 2 || rangeResp.Summary.UniqueUsers != 1 {
		t.Fatalf("range summary = %+v", rangeResp.Summary)
	}

	// Swapped from/to still works (normalized).
	swapReq := httptest.NewRequest(http.MethodGet, "/admin/api/task-reports?from="+to+"&to="+from, nil)
	swapRec := httptest.NewRecorder()
	h.ListTaskReports(swapRec, swapReq)
	var swapResp struct {
		From    string                  `json:"from"`
		To      string                  `json:"to"`
		Summary store.TaskReportSummary `json:"summary"`
	}
	_ = json.Unmarshal(swapRec.Body.Bytes(), &swapResp)
	if swapResp.From != from || swapResp.To != to || swapResp.Summary.TotalTasks != 2 {
		t.Fatalf("swapped = %s..%s summary=%+v", swapResp.From, swapResp.To, swapResp.Summary)
	}

	// Legacy date= still works as single day.
	y := yesterday.Format("2006-01-02")
	legacy := httptest.NewRequest(http.MethodGet, "/admin/api/task-reports?date="+y, nil)
	legacyRec := httptest.NewRecorder()
	h.ListTaskReports(legacyRec, legacy)
	var legacyResp struct {
		From    string                  `json:"from"`
		To      string                  `json:"to"`
		Summary store.TaskReportSummary `json:"summary"`
	}
	_ = json.Unmarshal(legacyRec.Body.Bytes(), &legacyResp)
	if legacyResp.From != y || legacyResp.To != y || legacyResp.Summary.TotalTasks != 1 {
		t.Fatalf("legacy date = %s..%s summary=%+v", legacyResp.From, legacyResp.To, legacyResp.Summary)
	}

	// from=all → everything.
	allReq := httptest.NewRequest(http.MethodGet, "/admin/api/task-reports?from=all", nil)
	allRec := httptest.NewRecorder()
	h.ListTaskReports(allRec, allReq)
	var allResp struct {
		From    string                  `json:"from"`
		To      string                  `json:"to"`
		Summary store.TaskReportSummary `json:"summary"`
	}
	_ = json.Unmarshal(allRec.Body.Bytes(), &allResp)
	if allResp.From != "all" || allResp.Summary.TotalTasks != 3 {
		t.Fatalf("all = %s..%s summary=%+v", allResp.From, allResp.To, allResp.Summary)
	}

	bad := httptest.NewRequest(http.MethodGet, "/admin/api/task-reports?from=not-a-day&to="+to, nil)
	badRec := httptest.NewRecorder()
	h.ListTaskReports(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("bad date status = %d, want 400", badRec.Code)
	}
}

func boolJSON(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func artifactJSON(n int) string {
	if n <= 0 {
		return "[]"
	}
	parts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		parts = append(parts, `"f`+itoa(i)+`.md"`)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
