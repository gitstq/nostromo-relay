package ws

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func netPipe(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	type res struct {
		c   net.Conn
		err error
	}
	ch := make(chan res, 1)
	go func() {
		c, err := ln.Accept()
		ch <- res{c, err}
	}()
	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	r := <-ch
	if r.err != nil {
		t.Fatal(r.err)
	}
	return r.c, client
}

// ---- minimal WebSocket client used only by tests ----

func dialWS(t *testing.T, serverURL string) net.Conn {
	t.Helper()
	addr := strings.TrimPrefix(serverURL, "http://")
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	key := base64.StdEncoding.EncodeToString(nonce)
	req := "GET / HTTP/1.1\r\nHost: " + addr + "\r\nUpgrade: websocket\r\n" +
		"Connection: Upgrade\r\nSec-WebSocket-Key: " + key + "\r\nSec-WebSocket-Version: 13\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(conn)
	resp, err := br.ReadString('\n')
	if err != nil || !strings.Contains(resp, "101") {
		t.Fatalf("bad status: %q", resp)
	}
	var gotAccept string
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "sec-websocket-accept:") {
			gotAccept = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}
	}
	h := sha1.New()
	h.Write([]byte(key))
	h.Write([]byte(websocketGUID))
	want := base64.StdEncoding.EncodeToString(h.Sum(nil))
	if gotAccept != want {
		t.Fatalf("accept mismatch got %q want %q", gotAccept, want)
	}
	return conn
}

func clientWrite(t *testing.T, conn net.Conn, fin bool, opcode byte, payload []byte) {
	t.Helper()
	var b0 byte
	if fin {
		b0 = 0x80
	}
	b0 |= opcode
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		t.Fatal(err)
	}
	masked := make([]byte, len(payload))
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	var header [10]byte
	n := 0
	header[0] = b0
	l := len(payload)
	switch {
	case l <= 125:
		header[1] = 0x80 | byte(l)
		n = 2
	case l <= 65535:
		header[1] = 0x80 | extended16
		binary.BigEndian.PutUint16(header[2:4], uint16(l))
		n = 4
	default:
		header[1] = 0x80 | extended64
		binary.BigEndian.PutUint64(header[2:10], uint64(l))
		n = 10
	}
	out := append(header[:n], mask[:]...)
	out = append(out, masked...)
	if _, err := conn.Write(out); err != nil {
		t.Fatal(err)
	}
}

func clientRead(t *testing.T, conn net.Conn) (bool, byte, []byte) {
	t.Helper()
	br := bufio.NewReader(conn)
	h := make([]byte, 2)
	if _, err := io.ReadFull(br, h); err != nil {
		t.Fatal(err)
	}
	fin := h[0]&0x80 != 0
	opcode := h[0] & 0x0F
	l := int64(h[1] & 0x7F)
	if l == extended16 {
		ext := make([]byte, 2)
		io.ReadFull(br, ext)
		l = int64(binary.BigEndian.Uint16(ext))
	} else if l == extended64 {
		ext := make([]byte, 8)
		io.ReadFull(br, ext)
		l = int64(binary.BigEndian.Uint64(ext))
	}
	payload := make([]byte, l)
	if l > 0 {
		io.ReadFull(br, payload)
	}
	return fin, opcode, payload
}

func TestEndToEndEcho(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrade(w, r, DefaultMaxMessageSize)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			op, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(op, data); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	conn := dialWS(t, srv.URL)
	defer conn.Close()

	clientWrite(t, conn, true, OpText, []byte("nostromo"))
	fin, opcode, payload := clientRead(t, conn)
	if !fin || opcode != OpText || string(payload) != "nostromo" {
		t.Fatalf("echo got fin=%v op=%d data=%q", fin, opcode, payload)
	}
}

func TestPingPongAndFragmentation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrade(w, r, DefaultMaxMessageSize)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()
	conn := dialWS(t, srv.URL)
	defer conn.Close()

	// Ping must produce an automatic pong with the same payload.
	clientWrite(t, conn, true, OpPing, []byte("hb"))
	fin, opcode, payload := clientRead(t, conn)
	if !fin || opcode != OpPong || string(payload) != "hb" {
		t.Fatalf("pong got fin=%v op=%d payload=%q", fin, opcode, payload)
	}

	// Fragmented text in two frames assembles into one message. Echo server
	// would echo; instead verify server-side assembly via a dedicated server.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrade(w, r, DefaultMaxMessageSize)
		if err != nil {
			return
		}
		defer conn.Close()
		op, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(op, data)
	}))
	defer srv2.Close()
	conn2 := dialWS(t, srv2.URL)
	defer conn2.Close()
	clientWrite(t, conn2, false, OpText, []byte("foo"))
	clientWrite(t, conn2, true, OpContinuation, []byte("bar"))
	_, opcode, payload = clientRead(t, conn2)
	if opcode != OpText || string(payload) != "foobar" {
		t.Fatalf("fragment assembly got op=%d payload=%q", opcode, payload)
	}
}

func TestMessageTooBig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrade(w, r, 16) // tiny limit
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()
	conn := dialWS(t, srv.URL)
	defer conn.Close()
	clientWrite(t, conn, true, OpText, make([]byte, 64))
	fin, opcode, payload := clientRead(t, conn)
	_ = fin
	if opcode != OpClose {
		t.Fatalf("expected close frame, got opcode %d", opcode)
	}
	if len(payload) >= 2 && binary.BigEndian.Uint16(payload[0:2]) != StatusMessageTooBig {
		t.Fatalf("expected 1009, got %d", binary.BigEndian.Uint16(payload[0:2]))
	}
}

func Example_writeFrame() {
	fmt.Println("see tests")
	// Output: see tests
}
