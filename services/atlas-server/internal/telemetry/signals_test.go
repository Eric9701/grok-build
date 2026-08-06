package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/atlas-build/atlas-server/internal/store"
	"github.com/go-chi/chi/v5"
)

type fakeSignalsStore struct {
	rows []store.SessionSignalsRecord
}

func (f *fakeSignalsStore) InsertSessionSignals(r store.SessionSignalsRecord) (int64, error) {
	r.ID = int64(len(f.rows) + 1)
	f.rows = append(f.rows, r)
	return r.ID, nil
}

func (f *fakeSignalsStore) ListSessionSignals(userID, sessionID string, limit int) ([]store.SessionSignalsRecord, error) {
	var out []store.SessionSignalsRecord
	for i := len(f.rows) - 1; i >= 0; i-- {
		r := f.rows[i]
		if userID != "" && r.UserID != userID {
			continue
		}
		if sessionID != "" && r.SessionID != sessionID {
			continue
		}
		out = append(out, r)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func withSessionID(req *http.Request, sessionID string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("sessionId", sessionID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

func TestUpdateSessionSignalsStoresPayload(t *testing.T) {
	st := &fakeSignalsStore{}
	h := NewHandler(nil, nil, nil, st, nil)

	body := `{
		"clientType":"tui",
		"totalTurns":3,
		"toolCallCount":12,
		"errorCount":1,
		"primaryModelId":"grok-code",
		"toolsUsed":["Bash","Read"]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/sess-1/signals", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-userid", "atlas-test-user")
	req = withSessionID(req, "sess-1")

	rr := httptest.NewRecorder()
	h.UpdateSessionSignals(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["sessionId"] != "sess-1" {
		t.Fatalf("sessionId=%v", resp["sessionId"])
	}
	if _, ok := resp["updatedAt"].(string); !ok {
		t.Fatalf("updatedAt missing: %v", resp)
	}
	if len(st.rows) != 1 {
		t.Fatalf("stored=%d", len(st.rows))
	}
	got := st.rows[0]
	if got.SessionID != "sess-1" || got.UserID != "atlas-test-user" {
		t.Fatalf("record=%+v", got)
	}
	if got.TotalTurns != 3 || got.ToolCallCount != 12 || got.ClientType != "tui" {
		t.Fatalf("indexed fields=%+v", got)
	}
	if got.PrimaryModelID != "grok-code" {
		t.Fatalf("primaryModelId=%q", got.PrimaryModelID)
	}
}

func TestListSessionSignalsFilters(t *testing.T) {
	st := &fakeSignalsStore{}
	h := NewHandler(nil, nil, nil, st, nil)

	_, _ = st.InsertSessionSignals(store.SessionSignalsRecord{
		UserID: "u1", SessionID: "s1", TotalTurns: 1, Payload: json.RawMessage(`{}`),
	})
	_, _ = st.InsertSessionSignals(store.SessionSignalsRecord{
		UserID: "u1", SessionID: "s2", TotalTurns: 2, Payload: json.RawMessage(`{}`),
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/api/session-signals?user_id=u1&session_id=s2", nil)
	rr := httptest.NewRecorder()
	h.ListSessionSignals(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp struct {
		Count   int              `json:"count"`
		Signals []map[string]any `json:"signals"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count != 1 || resp.Signals[0]["sessionId"] != "s2" {
		t.Fatalf("resp=%+v", resp)
	}
}
