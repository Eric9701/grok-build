package main

import (
	"encoding/json"
	"testing"
	"time"
)

func testPeer(h *hub, r role, id string) *peer {
	return &peer{
		role: r,
		id:   id,
		send: make(chan []byte, 16),
		hub:  h,
	}
}

func drain(p *peer) {
	for {
		select {
		case <-p.send:
		default:
			return
		}
	}
}

func lastStatus(t *testing.T, p *peer) statusParams {
	t.Helper()
	deadline := time.Now().Add(200 * time.Millisecond)
	var last statusParams
	got := false
	for time.Now().Before(deadline) {
		select {
		case raw := <-p.send:
			var msg struct {
				Method string       `json:"method"`
				Params statusParams `json:"params"`
			}
			if json.Unmarshal(raw, &msg) != nil || msg.Method != "atlas.relay/status" {
				continue
			}
			last = msg.Params
			got = true
		default:
			if got {
				return last
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	if !got {
		t.Fatal("no status frame")
	}
	return last
}

func TestHubListsAgentsByIdentity(t *testing.T) {
	h := newHub()
	c := testPeer(h, roleClient, "")
	h.attach(c)
	h.attach(testPeer(h, roleAgent, "laptop-a"))
	h.attach(testPeer(h, roleAgent, "laptop-b"))
	st := lastStatus(t, c)
	if len(st.Agents) != 2 {
		t.Fatalf("agents=%d want 2: %+v", len(st.Agents), st.Agents)
	}
}

func TestSameIdentityReplacesAgent(t *testing.T) {
	h := newHub()
	old := testPeer(h, roleAgent, "laptop-a")
	h.attach(old)
	h.attach(testPeer(h, roleAgent, "laptop-a"))
	views := h.agentViews()
	if len(views) != 1 || views[0].ID != "laptop-a" {
		t.Fatalf("views=%+v", views)
	}
	select {
	case _, ok := <-old.send:
		if ok {
			t.Fatal("replaced agent send still open")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("replaced agent was not closed")
	}
}

func TestBindForwardsOnlyToBoundPair(t *testing.T) {
	h := newHub()
	a := testPeer(h, roleAgent, "laptop-a")
	b := testPeer(h, roleAgent, "laptop-b")
	c := testPeer(h, roleClient, "")
	h.attach(a)
	h.attach(b)
	h.attach(c)
	h.bindClient(c, "laptop-a")
	drain(c)
	h.forward(a, []byte(`{"from":"a"}`))
	h.forward(b, []byte(`{"from":"b"}`))
	got := string(<-c.send)
	if got != `{"from":"a"}` {
		t.Fatalf("got %s", got)
	}
	select {
	case extra := <-c.send:
		t.Fatalf("unexpected extra %s", extra)
	default:
	}
}

func TestSecondClientStealsBind(t *testing.T) {
	h := newHub()
	a := testPeer(h, roleAgent, "laptop-a")
	c1 := testPeer(h, roleClient, "")
	c2 := testPeer(h, roleClient, "")
	h.attach(a)
	h.attach(c1)
	drain(c1)
	h.bindClient(c1, "laptop-a")
	h.attach(c2)
	h.bindClient(c2, "laptop-a")
	if h.boundOf(c1) == "laptop-a" {
		t.Fatal("first client still bound")
	}
	if h.boundOf(c2) != "laptop-a" {
		t.Fatalf("second client bound=%q", h.boundOf(c2))
	}
}

func TestAgentDisconnectKeepsBind(t *testing.T) {
	h := newHub()
	a := testPeer(h, roleAgent, "laptop-a")
	c := testPeer(h, roleClient, "")
	h.attach(c)
	h.attach(a)
	h.bindClient(c, "laptop-a")
	h.detach(a)
	if h.boundOf(c) != "laptop-a" {
		t.Fatalf("bind lost: %q", h.boundOf(c))
	}
	st := lastStatus(t, c)
	if st.BoundAgentID != "laptop-a" {
		t.Fatalf("status bind=%q", st.BoundAgentID)
	}
	online := false
	for _, v := range st.Agents {
		if v.ID == "laptop-a" && v.Online {
			online = true
		}
	}
	if online {
		t.Fatal("disconnected agent still online")
	}
}

func TestAutoBindOnlyWhenUnclaimed(t *testing.T) {
	h := newHub()
	a := testPeer(h, roleAgent, "laptop-a")
	c1 := testPeer(h, roleClient, "")
	c2 := testPeer(h, roleClient, "")
	h.attach(a)
	h.attach(c1)
	if h.boundOf(c1) != "laptop-a" {
		t.Fatalf("first client should auto-bind, got %q", h.boundOf(c1))
	}
	h.attach(c2)
	if h.boundOf(c2) != "" {
		t.Fatalf("second client auto-bound to claimed agent: %q", h.boundOf(c2))
	}
}
