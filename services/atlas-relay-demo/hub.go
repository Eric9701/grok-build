package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 15 * time.Second
	maxMessageSize = 8 << 20
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  64 << 10,
	WriteBufferSize: 64 << 10,
	CheckOrigin:     func(*http.Request) bool { return true },
}

type role string

const (
	roleAgent  role = "agent"
	roleClient role = "client"
)

type peer struct {
	role      role
	id        string
	userID    string
	version   string
	conn      *websocket.Conn
	send      chan []byte
	hub       *hub
	closeOnce sync.Once
}

type agentRec struct {
	id      string
	userID  string
	version string
	peer    *peer
}

type clientRec struct {
	bound string
}

type agentView struct {
	ID      string `json:"id"`
	UserID  string `json:"userId,omitempty"`
	Version string `json:"version,omitempty"`
	Online  bool   `json:"online"`
}

type statusParams struct {
	Agents       []agentView `json:"agents"`
	BoundAgentID string      `json:"boundAgentId,omitempty"`
}

type hub struct {
	mu      sync.Mutex
	agents  map[string]*agentRec
	claims  map[string]*peer
	clients map[*peer]*clientRec
}

func newHub() *hub {
	return &hub{
		agents:  make(map[string]*agentRec),
		claims:  make(map[string]*peer),
		clients: make(map[*peer]*clientRec),
	}
}

func (h *hub) attach(p *peer) {
	switch p.role {
	case roleAgent:
		h.attachAgent(p)
	case roleClient:
		h.attachClient(p)
	}
}

func (h *hub) attachAgent(p *peer) {
	h.mu.Lock()
	rec := h.agents[p.id]
	if rec == nil {
		rec = &agentRec{id: p.id}
		h.agents[p.id] = rec
	}
	old := rec.peer
	rec.peer = p
	rec.userID = p.userID
	rec.version = p.version
	if len(h.onlineIDsLocked()) == 1 {
		for c, cr := range h.clients {
			if cr.bound == "" && h.claims[p.id] == nil {
				h.bindLocked(c, p.id)
			}
		}
	}
	n := len(h.onlineIDsLocked())
	h.mu.Unlock()

	if old != nil && old != p {
		old.close("replaced")
	}
	log.Printf("agent %s connected (online=%d)", p.id, n)
	h.pushStatus()
}

func (h *hub) attachClient(p *peer) {
	h.mu.Lock()
	h.clients[p] = &clientRec{}
	online := h.onlineIDsLocked()
	if len(online) == 1 && h.claims[online[0]] == nil {
		h.bindLocked(p, online[0])
	}
	bound := h.clients[p].bound
	h.mu.Unlock()
	log.Printf("client connected (bound=%q)", bound)
	h.pushStatus()
}

func (h *hub) bindClient(p *peer, agentID string) {
	agentID = cleanID(agentID)
	if agentID == "" {
		return
	}
	h.mu.Lock()
	h.bindLocked(p, agentID)
	h.mu.Unlock()
	log.Printf("client bound to %s", agentID)
	h.pushStatus()
}

func (h *hub) bindLocked(c *peer, agentID string) {
	cr := h.clients[c]
	if cr == nil {
		return
	}
	if cr.bound != "" && cr.bound != agentID && h.claims[cr.bound] == c {
		delete(h.claims, cr.bound)
	}
	if prev := h.claims[agentID]; prev != nil && prev != c {
		if pcr := h.clients[prev]; pcr != nil {
			pcr.bound = ""
		}
	}
	cr.bound = agentID
	h.claims[agentID] = c
}

func (h *hub) detach(p *peer) {
	h.mu.Lock()
	switch p.role {
	case roleAgent:
		if rec := h.agents[p.id]; rec != nil && rec.peer == p {
			rec.peer = nil
		}
		log.Printf("agent %s disconnected", p.id)
	case roleClient:
		if cr := h.clients[p]; cr != nil {
			if cr.bound != "" && h.claims[cr.bound] == p {
				delete(h.claims, cr.bound)
			}
			delete(h.clients, p)
		}
		log.Printf("client disconnected")
	}
	h.mu.Unlock()
	h.pushStatus()
}

func (h *hub) onlineIDsLocked() []string {
	var ids []string
	for id, rec := range h.agents {
		if rec.peer != nil {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func (h *hub) agentViews() []agentView {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.onlineViewsLocked()
}

func (h *hub) onlineViewsLocked() []agentView {
	var out []agentView
	for id, rec := range h.agents {
		if rec.peer == nil {
			continue
		}
		out = append(out, agentView{
			ID: id, UserID: rec.userID, Version: rec.version, Online: true,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (h *hub) boundOf(p *peer) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cr := h.clients[p]; cr != nil {
		return cr.bound
	}
	return ""
}

func (h *hub) statusForLocked(c *peer) statusParams {
	seen := map[string]bool{}
	agents := h.onlineViewsLocked()
	for _, a := range agents {
		seen[a.ID] = true
	}
	bound := ""
	if cr := h.clients[c]; cr != nil {
		bound = cr.bound
		if bound != "" && !seen[bound] {
			view := agentView{ID: bound, Online: false}
			if rec := h.agents[bound]; rec != nil {
				view.UserID = rec.userID
				view.Version = rec.version
			}
			agents = append(agents, view)
			sort.Slice(agents, func(i, j int) bool { return agents[i].ID < agents[j].ID })
		}
	}
	return statusParams{Agents: agents, BoundAgentID: bound}
}

func (h *hub) snapshot() map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	return map[string]any{
		"agents": h.onlineViewsLocked(),
	}
}

func (h *hub) forward(from *peer, payload []byte) {
	h.mu.Lock()
	var dest *peer
	switch from.role {
	case roleAgent:
		if h.claims[from.id] != nil {
			dest = h.claims[from.id]
		}
	case roleClient:
		if cr := h.clients[from]; cr != nil && cr.bound != "" {
			if rec := h.agents[cr.bound]; rec != nil {
				dest = rec.peer
			}
		}
	}
	h.mu.Unlock()
	if dest == nil {
		return
	}
	if !trySend(dest.send, payload) {
		log.Printf("drop frame: %s inbox full or closed", dest.role)
	}
}

func (h *hub) pushStatus() {
	h.mu.Lock()
	clients := make([]*peer, 0, len(h.clients))
	payloads := make([][]byte, 0, len(h.clients))
	for c := range h.clients {
		body, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"method":  "atlas.relay/status",
			"params":  h.statusForLocked(c),
		})
		clients = append(clients, c)
		payloads = append(payloads, body)
	}
	h.mu.Unlock()
	for i, c := range clients {
		_ = trySend(c.send, payloads[i])
	}
}

func trySend(ch chan []byte, msg []byte) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	select {
	case ch <- msg:
		return true
	default:
		return false
	}
}

func (p *peer) close(_ string) {
	if p == nil {
		return
	}
	p.closeOnce.Do(func() {
		close(p.send)
		if p.conn != nil {
			_ = p.conn.Close()
		}
	})
}

func (p *peer) readPump() {
	defer func() {
		p.close("read-end")
		p.hub.detach(p)
	}()
	p.conn.SetReadLimit(maxMessageSize)
	_ = p.conn.SetReadDeadline(time.Now().Add(pongWait))
	p.conn.SetPongHandler(func(string) error {
		return p.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		mt, data, err := p.conn.ReadMessage()
		if err != nil {
			return
		}
		if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
			continue
		}
		text := trimFrame(data)
		if text == nil {
			continue
		}
		if p.role == roleClient {
			if method, params, ok := parseRelayControl(text); ok {
				if method == "atlas.relay/bind" {
					id, _ := params["agentId"].(string)
					if id == "" {
						id, _ = params["agent_id"].(string)
					}
					p.hub.bindClient(p, id)
				}
				continue
			}
		}
		p.hub.forward(p, text)
	}
}

func (p *peer) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		if p.conn != nil {
			_ = p.conn.Close()
		}
	}()
	for {
		select {
		case msg, ok := <-p.send:
			_ = p.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = p.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := p.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = p.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := p.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func trimFrame(data []byte) []byte {
	i, j := 0, len(data)
	for i < j && (data[i] == '\n' || data[i] == '\r' || data[i] == ' ') {
		i++
	}
	for j > i && (data[j-1] == '\n' || data[j-1] == '\r') {
		j--
	}
	if i >= j {
		return nil
	}
	out := data[i:j]
	if len(out) == 4 && string(out) == "ping" {
		return nil
	}
	return append([]byte(nil), out...)
}

func parseRelayControl(data []byte) (method string, params map[string]any, ok bool) {
	var msg struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if json.Unmarshal(data, &msg) != nil {
		return "", nil, false
	}
	if len(msg.Method) < 12 || msg.Method[:12] != "atlas.relay/" {
		return "", nil, false
	}
	var p map[string]any
	if len(msg.Params) > 0 {
		_ = json.Unmarshal(msg.Params, &p)
	}
	return msg.Method, p, true
}
