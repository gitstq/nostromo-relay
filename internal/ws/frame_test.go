package ws

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestWriteFrameFormats(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		first   byte
		extLen  int
	}{
		{"small", make([]byte, 100), 0x81, 0},
		{"16bit", make([]byte, 300), 0x81, 2},
		{"64bit", make([]byte, 70000), 0x81, 8},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		if err := writeFrame(&buf, false, OpText, tc.payload); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		b := buf.Bytes()
		if b[0] != tc.first {
			t.Fatalf("%s: fin/opcode byte %08b", tc.name, b[0])
		}
		if b[1]&0x80 != 0 {
			t.Fatalf("%s: server frame must not be masked", tc.name)
		}
		switch tc.extLen {
		case 2:
			if b[1]&0x7F != extended16 || binary.BigEndian.Uint16(b[2:4]) != 300 {
				t.Fatalf("%s: 16-bit length wrong", tc.name)
			}
		case 8:
			if b[1]&0x7F != extended64 || binary.BigEndian.Uint64(b[2:10]) != 70000 {
				t.Fatalf("%s: 64-bit length wrong", tc.name)
			}
		default:
			if b[1]&0x7F != 100 {
				t.Fatalf("%s: small length wrong", tc.name)
			}
		}
	}
}

func TestReadMaskedFrame(t *testing.T) {
	// Build a masked text frame "hello".
	payload := []byte("hello")
	mask := [4]byte{0x12, 0x34, 0x56, 0x78}
	masked := make([]byte, len(payload))
	for i := range payload {
		masked[i] = payload[i] ^ mask[i%4]
	}
	var frame bytes.Buffer
	frame.WriteByte(0x81) // FIN text
	frame.WriteByte(0x80 | byte(len(payload)))
	frame.Write(mask[:])
	frame.Write(masked)

	// Serve via a real TCP loopback connection.
	server, client := netPipe(t)
	defer server.Close()
	defer client.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn := newConn(server, 1024)
		op, data, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read: %v", err)
			return
		}
		if op != OpText || string(data) != "hello" {
			t.Errorf("got op=%d data=%q", op, data)
		}
	}()
	if _, err := client.Write(frame.Bytes()); err != nil {
		t.Fatal(err)
	}
	<-done
}
