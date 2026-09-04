package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sampleJob() dispatchJob {
	return dispatchJob{
		JobID:           "demo-1",
		RequirementsURL: "https://example.com/req.md",
		DesignURL:       "https://example.com/design.md",
		AcceptanceURL:   "https://example.com/ac.md",
	}
}

func TestBuildDispatchWritesContractPaths(t *testing.T) {
	p, err := buildDispatch(sampleJob())
	if err != nil {
		t.Fatal(err)
	}
	if p.RequirementsPath != "documents/requirements-analyst/demo-1-requirements.md" {
		t.Fatalf("req path %q", p.RequirementsPath)
	}
	if p.DesignPath != "documents/detailed-design/demo-1-detailed-design.md" {
		t.Fatalf("design path %q", p.DesignPath)
	}
	if p.AcceptancePath != "documents/test-cases/demo-1-acceptance.md" {
		t.Fatalf("accept path %q", p.AcceptancePath)
	}
	if p.ReportPath != "documents/execution-report-demo-1.json" {
		t.Fatalf("report path %q", p.ReportPath)
	}
}

func TestBuildDispatchDropUsesURLsNotInlineBody(t *testing.T) {
	p, err := buildDispatch(sampleJob())
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{
		"https://example.com/req.md",
		"https://example.com/design.md",
		"https://example.com/ac.md",
	} {
		if !strings.Contains(p.Drop, u) {
			t.Fatalf("drop missing %s", u)
		}
	}
	if !strings.Contains(p.Drop, "不要写业务代码") {
		t.Fatal("drop should forbid implementation")
	}
}

func TestBuildDispatchExecuteSpawnsRole4(t *testing.T) {
	p, err := buildDispatch(sampleJob())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Execute, role4Agent) {
		t.Fatal("execute must name Role 4 agent")
	}
	if !strings.Contains(p.Execute, "不要拉 Role 1") {
		t.Fatal("execute must skip earlier roles")
	}
	if strings.Contains(p.Execute, "https://example.com") {
		t.Fatal("execute should use disk paths, not fetch URLs again")
	}
}

func TestBuildDispatchRejectsNonHTTP(t *testing.T) {
	job := sampleJob()
	job.RequirementsURL = "file:///tmp/req.md"
	if _, err := buildDispatch(job); err == nil {
		t.Fatal("expected error")
	}
}

func TestBuildDispatchCleansJobID(t *testing.T) {
	job := sampleJob()
	job.JobID = " foo/bar "
	p, err := buildDispatch(job)
	if err != nil {
		t.Fatal(err)
	}
	if p.JobID != "foo-bar" {
		t.Fatalf("jobId=%q", p.JobID)
	}
}

func TestDispatchPromptsHTTP(t *testing.T) {
	body, _ := json.Marshal(sampleJob())
	req := httptest.NewRequest(http.MethodPost, "/dispatch/prompts", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	serveDispatchPrompts(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	var p dispatchPrompts
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.JobID != "demo-1" || !strings.Contains(p.Execute, role4Agent) {
		t.Fatalf("%+v", p)
	}
}

func TestParseExecutionReportFromFencedText(t *testing.T) {
	raw := "好的\n```json\n{\"jobId\":\"demo-1\",\"status\":\"passed\"}\n```\n"
	got, err := parseExecutionReport(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got["jobId"] != "demo-1" || got["status"] != "passed" {
		t.Fatalf("%v", got)
	}
}
