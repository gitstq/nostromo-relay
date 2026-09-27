package ws

import (
	"crypto/sha1"
	"encoding/base64"
	"net/http"
	"strings"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// DefaultMaxMessageSize is the cap on an assembled message (1 MiB). Nostr
// events are small JSON; the limit also bounds fragmentation memory.
const DefaultMaxMessageSize = 1 << 20

// Upgrade performs the WebSocket opening handshake and returns a framed Conn.
// On failure it writes an appropriate HTTP error and returns an error.
func Upgrade(w http.ResponseWriter, r *http.Request, maxMessageSize int64) (*Conn, error) {
	if !headerContains(r.Header, "Connection", "upgrade") {
		return fail(w, http.StatusBadRequest, "missing Connection: Upgrade")
	}
	if !headerContains(r.Header, "Upgrade", "websocket") {
		return fail(w, http.StatusBadRequest, "missing Upgrade: websocket")
	}
	if r.Method != http.MethodGet {
		return fail(w, http.StatusMethodNotAllowed, "WebSocket requires GET")
	}
	if r.Header.Get("Sec-WebSocket-Version") != "13" {
		w.Header().Set("Sec-WebSocket-Version", "13")
		return fail(w, http.StatusBadRequest, "unsupported WebSocket version")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if len(key) == 0 {
		return fail(w, http.StatusBadRequest, "missing Sec-WebSocket-Key")
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		return fail(w, http.StatusInternalServerError, "server does not support hijacking")
	}
	nc, brw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	accept := computeAccept(key)
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := brw.WriteString(response); err != nil {
		nc.Close()
		return nil, err
	}
	if err := brw.Flush(); err != nil {
		nc.Close()
		return nil, err
	}
	return newConn(nc, maxMessageSize), nil
}

func computeAccept(key string) string {
	h := sha1.New()
	h.Write([]byte(key))
	h.Write([]byte(websocketGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func headerContains(h http.Header, name, token string) bool {
	for _, v := range h[name] {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

func fail(w http.ResponseWriter, code int, msg string) (*Conn, error) {
	http.Error(w, msg, code)
	return nil, &handshakeError{msg: msg}
}

type handshakeError struct{ msg string }

func (e *handshakeError) Error() string { return "ws: handshake failed: " + e.msg }
