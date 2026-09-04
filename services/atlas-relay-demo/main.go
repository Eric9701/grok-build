// Atlas relay demo: pairs outbound `atlas agent headless` WebSockets
// with a browser chat page. Does not modify atlas-server.
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed web/index.html
var chatHTML []byte

//go:embed web/sample-docs
var sampleDocs embed.FS

func serveWS(h *hub, r role) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		conn, err := upgrader.Upgrade(w, req, nil)
		if err != nil {
			log.Printf("upgrade %s: %v", r, err)
			return
		}
		p := &peer{
			role:    r,
			conn:    conn,
			send:    make(chan []byte, 64),
			hub:     h,
			userID:  req.Header.Get("x-userid"),
			version: req.Header.Get("x-grok-client-version"),
		}
		if r == roleAgent {
			p.id = resolveAgentIdentity(req.URL.Query().Get("agent_id"), p.userID)
			log.Printf("agent register id=%q userid=%q ver=%q token-auth=%q",
				p.id, p.userID, p.version, req.Header.Get("X-XAI-Token-Auth"))
		}
		h.attach(p)
		go p.writePump()
		p.readPump()
	}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:2420", "listen address")
	flag.Parse()

	h := newHub()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/status", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(h.snapshot())
	})
	sampleRoot, err := fs.Sub(sampleDocs, "web/sample-docs")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/sample-docs/", http.StripPrefix("/sample-docs/", http.FileServer(http.FS(sampleRoot))))
	mux.HandleFunc("/dispatch/prompts", serveDispatchPrompts)
	mux.HandleFunc("/ws", serveWS(h, roleAgent))
	mux.HandleFunc("/ws/agent", serveWS(h, roleAgent))
	mux.HandleFunc("/ws/client", serveWS(h, roleClient))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/build" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(chatHTML)
	})

	fmt.Fprintf(os.Stderr, `
Atlas relay demo  http://%s/build

CLI（出站，建议带身份）:
  atlas agent --always-approve headless ^
    --grok-ws-url "ws://%s/ws?agent_id=laptop-a" ^
    --grok-ws-origin http://%s

浏览器打开 /build，可在顶栏选择 Agent。不要改 atlas-server。

`, *addr, *addr, *addr)

	log.Fatal(http.ListenAndServe(*addr, mux))
}
