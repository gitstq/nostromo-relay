package relay_test

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gitstq/nostromo-relay/internal/crypto"
	"github.com/gitstq/nostromo-relay/internal/protocol"
	"github.com/gitstq/nostromo-relay/internal/relay"
	"github.com/gitstq/nostromo-relay/internal/server"
	"github.com/gitstq/nostromo-relay/internal/store"
)

// ---- tiny WebSocket client for integration tests ----

type testClient struct {
	conn net.Conn
	br   *bufio.Reader
}

func dial(t *testing.T, u string) *testClient {
	t.Helper()
	addr := strings.TrimPrefix(u, "http://")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, 16)
	rand.Read(nonce)
	key := base64.StdEncoding.EncodeToString(nonce)
	fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\n"+
		"Connection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n",
		addr, key)
	br := bufio.NewReader(conn)
	status, _ := br.ReadString('\n')
	if !strings.Contains(status, "101") {
		t.Fatalf("handshake: %q", status)
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
	}
	return &testClient{conn: conn, br: br}
}

func (c *testClient) send(t *testing.T, v interface{}) {
	t.Helper()
	b, _ := json.Marshal(v)
	var mask [4]byte
	rand.Read(mask[:])
	masked := make([]byte, len(b))
	for i := range b {
		masked[i] = b[i] ^ mask[i%4]
	}
	var header []byte
	switch {
	case len(b) <= 125:
		header = []byte{0x81, 0x80 | byte(len(b))}
	case len(b) <= 65535:
		header = make([]byte, 4)
		header[0], header[1] = 0x81, 0x80|126
		binary.BigEndian.PutUint16(header[2:4], uint16(len(b)))
	default:
		header = make([]byte, 10)
		header[0], header[1] = 0x81, 0x80|127
		binary.BigEndian.PutUint64(header[2:10], uint64(len(b)))
	}
	c.conn.Write(header)
	c.conn.Write(mask[:])
	c.conn.Write(masked)
}

func (c *testClient) recv(t *testing.T) []interface{} {
	t.Helper()
	h := make([]byte, 2)
	io.ReadFull(c.br, h)
	l := int64(h[1] & 0x7F)
	if l == 126 {
		ext := make([]byte, 2)
		io.ReadFull(c.br, ext)
		l = int64(binary.BigEndian.Uint16(ext))
	} else if l == 127 {
		ext := make([]byte, 8)
		io.ReadFull(c.br, ext)
		l = int64(binary.BigEndian.Uint64(ext))
	}
	p := make([]byte, l)
	if l > 0 {
		io.ReadFull(c.br, p)
	}
	var arr []interface{}
	if err := json.Unmarshal(p, &arr); err != nil {
		t.Fatalf("recv parse: %v: %q", err, p)
	}
	return arr
}

func (c *testClient) close() { c.conn.Close() }

// ---- helpers ----

func setup(t *testing.T, requireAuth bool) (*httptest.Server, *relay.Relay) {
	t.Helper()
	cfg := relay.Defaults()
	cfg.RequireAuth = requireAuth
	r := relay.New(store.New(), cfg)
	srv := httptest.NewServer(server.New(r, 1))
	t.Cleanup(srv.Close)
	return srv, r
}

func signedNote(t *testing.T, kind int, content string, tags protocol.Tags) (*big.Int, protocol.Event) {
	t.Helper()
	priv, _ := crypto.GeneratePrivateKey()
	e := protocol.Event{CreatedAt: time.Now().Unix(), Kind: kind, Tags: tags, Content: content}
	if err := protocol.SignEvent(&e, priv); err != nil {
		t.Fatal(err)
	}
	return priv, e
}

func expectOK(t *testing.T, c *testClient, want bool) {
	t.Helper()
	arr := c.recv(t)
	if arr[0] != "OK" || arr[2] != want {
		t.Fatalf("want OK %v, got %v", want, arr)
	}
}

// ---- tests ----

func TestPublishAndQuery(t *testing.T) {
	srv, _ := setup(t, false)
	c := dial(t, srv.URL)
	defer c.close()

	_, e := signedNote(t, 1, "hello integration", nil)
	c.send(t, []interface{}{"EVENT", e})
	expectOK(t, c, true)

	c.send(t, []interface{}{"REQ", "sub1", protocol.Filter{Kinds: []int{1}}})
	got := c.recv(t) // EVENT
	if got[0] != "EVENT" || got[1] != "sub1" {
		t.Fatalf("want EVENT, got %v", got[0])
	}
	ev := got[2].(map[string]interface{})
	if ev["content"] != "hello integration" {
		t.Fatal("content mismatch")
	}
	eose := c.recv(t)
	if eose[0] != "EOSE" {
		t.Fatalf("want EOSE, got %v", eose)
	}
}

func TestLiveBroadcast(t *testing.T) {
	srv, _ := setup(t, false)
	sub := dial(t, srv.URL)
	defer sub.close()
	pub := dial(t, srv.URL)
	defer pub.close()

	// Subscriber opens a live subscription.
	sub.send(t, []interface{}{"REQ", "live", protocol.Filter{Kinds: []int{1}}})
	if arr := sub.recv(t); arr[0] != "EOSE" {
		t.Fatalf("want initial EOSE, got %v", arr)
	}

	// Publisher sends a note.
	_, e := signedNote(t, 1, "live event", nil)
	pub.send(t, []interface{}{"EVENT", e})
	expectOK(t, pub, true)

	// Subscriber receives the event in real time.
	got := sub.recv(t)
	if got[0] != "EVENT" || got[1] != "live" {
		t.Fatalf("subscriber did not get live event: %v", got)
	}
}

func TestInvalidEventRejected(t *testing.T) {
	srv, _ := setup(t, false)
	c := dial(t, srv.URL)
	defer c.close()

	_, e := signedNote(t, 1, "good", nil)
	e.Sig = strings.Repeat("0", 128) // break signature
	c.send(t, []interface{}{"EVENT", e})
	expectOK(t, c, false)
}

func TestEphemeralBroadcastNotStored(t *testing.T) {
	srv, r := setup(t, false)
	sub := dial(t, srv.URL)
	defer sub.close()
	pub := dial(t, srv.URL)
	defer pub.close()

	sub.send(t, []interface{}{"REQ", "eph", protocol.Filter{Kinds: []int{20001}}})
	sub.recv(t) // EOSE

	_, e := signedNote(t, 20001, "ephemeral", nil)
	pub.send(t, []interface{}{"EVENT", e})
	expectOK(t, pub, true)

	got := sub.recv(t)
	if got[0] != "EVENT" {
		t.Fatalf("ephemeral not broadcast: %v", got)
	}
	if r.Store().Count() != 0 {
		t.Fatal("ephemeral event must not be stored")
	}
}

func TestAuthFlow(t *testing.T) {
	srv, _ := setup(t, true)
	c := dial(t, srv.URL)
	defer c.close()

	// Unauthenticated REQ is challenged and closed.
	c.send(t, []interface{}{"REQ", "secret", protocol.Filter{Kinds: []int{1}}})
	authMsg := c.recv(t)
	if authMsg[0] != "AUTH" {
		t.Fatalf("want AUTH challenge, got %v", authMsg)
	}
	closed := c.recv(t)
	if closed[0] != "CLOSED" {
		t.Fatalf("want CLOSED, got %v", closed)
	}

	// Build a valid auth event.
	priv, _ := crypto.GeneratePrivateKey()
	challenge := authMsg[1].(string)
	authEvent := protocol.Event{
		CreatedAt: time.Now().Unix(),
		Kind:      protocol.KindAuth,
		Tags: protocol.Tags{
			{"relay", "ws://relay"},
			{"challenge", challenge},
		},
	}
	if err := protocol.SignEvent(&authEvent, priv); err != nil {
		t.Fatal(err)
	}
	c.send(t, []interface{}{"AUTH", authEvent})
	expectOK(t, c, true)

	// Now a subscription is accepted.
	c.send(t, []interface{}{"REQ", "secret", protocol.Filter{Kinds: []int{1}}})
	if arr := c.recv(t); arr[0] != "EOSE" {
		t.Fatalf("post-auth REQ failed: %v", arr)
	}
}
