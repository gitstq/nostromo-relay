// Package server wires the relay to HTTP: the WebSocket endpoint, the NIP-11
// relay information document, health/metrics endpoints and the embedded
// developer console.
package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gitstq/nostromo-relay/internal/relay"
	"github.com/gitstq/nostromo-relay/internal/ws"
)

// Version is the semantic version of the relay.
const Version = "1.0.0"

//go:embed web
var webFS embed.FS

// supportedNIPs lists the NIPs implemented by this relay.
var supportedNIPs = []int{1, 2, 9, 11, 12, 16, 20, 28, 33, 40, 42, 50}

// Handler bundles the relay with HTTP routing.
type Handler struct {
	relay      *relay.Relay
	maxMsgSize int64
	static     http.Handler
}

// New constructs the HTTP handler.
func New(r *relay.Relay, maxMsgMB int64) *Handler {
	if maxMsgMB <= 0 {
		maxMsgMB = ws.DefaultMaxMessageSize
	} else {
		maxMsgMB *= 1024 * 1024
	}
	sub, _ := fs.Sub(webFS, "web")
	return &Handler{
		relay:      r,
		maxMsgSize: maxMsgMB,
		static:     http.FileServer(http.FS(sub)),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case isUpgrade(r):
		h.serveWS(w, r)
		return
	case path == "/healthz":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	case path == "/metrics":
		h.serveMetrics(w, r)
		return
	case path == "/" && r.Method == http.MethodGet:
		if strings.Contains(r.Header.Get("Accept"), "application/nostr+json") {
			h.serveNIP11(w, r)
			return
		}
	}
	h.static.ServeHTTP(w, r)
}

func (h *Handler) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.Upgrade(w, r, h.maxMsgSize)
	if err != nil {
		return
	}
	h.relay.Serve(conn, selfURL(r))
}

func (h *Handler) serveNIP11(w http.ResponseWriter, r *http.Request) {
	cfg := h.relay.Config()
	doc := map[string]interface{}{
		"name":           cfg.Name,
		"description":    cfg.Description,
		"pubkey":         cfg.PubKey,
		"contact":        cfg.Contact,
		"supported_nips": supportedNIPs,
		"software":       "https://github.com/gitstq/nostromo-relay",
		"version":        Version,
	}
	w.Header().Set("Content-Type", "application/nostr+json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(doc)
}

func (h *Handler) serveMetrics(w http.ResponseWriter, r *http.Request) {
	events, clients, accepted, rejected := h.relay.Stats()
	_ = json.NewEncoder(w).Encode(map[string]int64{
		"events":            events,
		"connected_clients": clients,
		"accepted_events":   accepted,
		"rejected_events":   rejected,
	})
}

func isUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

// selfURL reconstructs the externally visible ws/wss URL.
func selfURL(r *http.Request) string {
	scheme := "ws"
	if r.TLS != nil {
		scheme = "wss"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		if strings.HasPrefix(proto, "https") {
			scheme = "wss"
		} else {
			scheme = "ws"
		}
	}
	return scheme + "://" + r.Host
}
