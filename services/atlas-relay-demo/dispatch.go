package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const role4Agent = "atlas-sdd:4-software-engineer-agent"

var jobIDPat = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type dispatchJob struct {
	JobID           string `json:"jobId"`
	RequirementsURL string `json:"requirementsUrl"`
	DesignURL       string `json:"designUrl"`
	AcceptanceURL   string `json:"acceptanceUrl"`
}

type dispatchPrompts struct {
	JobID            string `json:"jobId"`
	RequirementsPath string `json:"requirementsPath"`
	DesignPath       string `json:"designPath"`
	AcceptancePath   string `json:"acceptancePath"`
	ReportPath       string `json:"reportPath"`
	Drop             string `json:"drop"`
	Execute          string `json:"execute"`
	Report           string `json:"report"`
}

func cleanJobID(s string) string {
	s = strings.TrimSpace(s)
	s = jobIDPat.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-.")
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

func httpURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("url must be http or https")
	}
	return raw, nil
}

func buildDispatch(job dispatchJob) (dispatchPrompts, error) {
	id := cleanJobID(job.JobID)
	if id == "" {
		return dispatchPrompts{}, fmt.Errorf("jobId required")
	}
	reqURL, err := httpURL(job.RequirementsURL)
	if err != nil {
		return dispatchPrompts{}, fmt.Errorf("requirementsUrl: %w", err)
	}
	designURL, err := httpURL(job.DesignURL)
	if err != nil {
		return dispatchPrompts{}, fmt.Errorf("designUrl: %w", err)
	}
	acceptURL, err := httpURL(job.AcceptanceURL)
	if err != nil {
		return dispatchPrompts{}, fmt.Errorf("acceptanceUrl: %w", err)
	}

	reqPath := "documents/requirements-analyst/" + id + "-requirements.md"
	designPath := "documents/detailed-design/" + id + "-detailed-design.md"
	acceptPath := "documents/test-cases/" + id + "-acceptance.md"
	reportPath := "documents/execution-report-" + id + ".json"

	p := dispatchPrompts{
		JobID:            id,
		RequirementsPath: reqPath,
		DesignPath:       designPath,
		AcceptancePath:   acceptPath,
		ReportPath:       reportPath,
		Drop: fmt.Sprintf(`派工 %s。只做落盘，不要写业务代码。

从 URL 下载文档（curl 或 web_fetch），写入当前仓库 cwd，已存在则覆盖：

1. %s
   → %s
2. %s
   → %s
3. %s
   → %s

创建缺失的目录。完成后只回复三个已写入的绝对路径。`,
			id, reqURL, reqPath, designURL, designPath, acceptURL, acceptPath),
		Execute: fmt.Sprintf(`派工 %s。Documents Contract 已在盘上，当前是实现阶段。

- 需求：%s
- 设计：%s
- 验收：%s

用 Task 拉起 %s，run_in_background 必须为 false。
不要拉 Role 1 / 2 / 3，不要跑完整 /implement。
告诉 Role 4：按详细设计实现；内部用 tdd（红绿切片）；对照验收文档跑能跑的检查。

Role 4 返回后，在本会话简要说明改了什么。`,
			id, reqPath, designPath, acceptPath, role4Agent),
		Report: fmt.Sprintf(`派工 %s 已结束实现。不要再改业务代码。

把回执写成 %s，字段必须是：

{"jobId":"%s","status":"passed|failed|blocked","summary":"","changedFiles":[],"tests":{"command":"","exitCode":0,"logExcerpt":""},"acceptance":[{"id":"","result":"pass|fail|skip","evidence":""}]}

根据仓库真实状态填写。写完后在回复里只输出这一份 JSON（可以包在单个 json 代码块里）。`,
			id, reportPath, id),
	}
	return p, nil
}

func parseExecutionReport(text string) (map[string]any, error) {
	text = strings.TrimSpace(text)
	if i := strings.Index(text, "{"); i >= 0 {
		if j := strings.LastIndex(text, "}"); j > i {
			text = text[i : j+1]
		}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return nil, err
	}
	if _, ok := out["jobId"]; !ok {
		return nil, fmt.Errorf("missing jobId")
	}
	return out, nil
}

func serveDispatchPrompts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var job dispatchJob
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&job); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	p, err := buildDispatch(job)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}
