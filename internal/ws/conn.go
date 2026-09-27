// Package ws is a minimal, dependency-free WebSocket (RFC 6455) implementation
// for the server side of the Nostr relay.
package ws

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Opcode / message type constants.
const (
	OpContinuation = 0x0
	OpText         = 0x1
	OpBinary       = 0x2
	OpClose        = 0x8
	OpPing         = 0x9
	OpPong         = 0xA
)

// Close status codes (RFC 6455 §7.4).
const (
	StatusNormalClosure   = 1000
	StatusProtocolError   = 1002
	StatusUnsupportedData = 1003
	StatusMessageTooBig   = 1009
	StatusPolicyViolation = 1008
	StatusInternalError   = 1011
)

const (
	smallPayload      = 125
	extended16        = 126
	extended64        = 127
	maxControlPayload = 125
)

var (
	// ErrCloseSent is returned when writing after a close frame.
	ErrCloseSent = errors.New("ws: close already sent")
)

// Conn wraps a network connection with WebSocket framing.
type Conn struct {
	nc             net.Conn
	writeMu        sync.Mutex
	maxMessageSize int64

	// read-side fragmentation state.
	fragActive bool
	fragOpcode byte
	fragBuf    bytes.Buffer

	closeOnce sync.Once
	closed    bool
}

func newConn(nc net.Conn, maxMessageSize int64) *Conn {
	return &Conn{nc: nc, maxMessageSize: maxMessageSize}
}

// SetDeadline proxies to the underlying connection.
func (c *Conn) SetDeadline(t time.Time) error { return c.nc.SetDeadline(t) }

// SetReadDeadline proxies to the underlying connection.
func (c *Conn) SetReadDeadline(t time.Time) error { return c.nc.SetReadDeadline(t) }

// SetWriteDeadline proxies to the underlying connection.
func (c *Conn) SetWriteDeadline(t time.Time) error { return c.nc.SetWriteDeadline(t) }

type frameHeader struct {
	fin     bool
	opcode  byte
	masked  bool
	payload []byte
}

// readFrame reads and decodes a single frame. Client frames must be masked.
func (c *Conn) readFrame() (*frameHeader, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.nc, header); err != nil {
		return nil, err
	}
	fin := header[0]&0x80 != 0
	rsv := header[0] & 0x70
	opcode := header[0] & 0x0F
	if rsv != 0 {
		return nil, protocolError("reserved bits must be zero")
	}
	masked := header[1]&0x80 != 0
	length := int64(header[1] & 0x7F)
	switch length {
	case extended16:
		ext := make([]byte, 2)
		if _, err := io.ReadFull(c.nc, ext); err != nil {
			return nil, err
		}
		length = int64(binary.BigEndian.Uint16(ext))
	case extended64:
		ext := make([]byte, 8)
		if _, err := io.ReadFull(c.nc, ext); err != nil {
			return nil, err
		}
		length = int64(binary.BigEndian.Uint64(ext))
	}
	if length < 0 {
		return nil, protocolError("negative payload length")
	}
	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(c.nc, maskKey[:]); err != nil {
			return nil, err
		}
	}
	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(c.nc, payload); err != nil {
			return nil, err
		}
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return &frameHeader{fin: fin, opcode: opcode, masked: masked, payload: payload}, nil
}

// ReadMessage returns the next fully assembled data message. Control frames are
// handled transparently (ping -> pong; close -> error).
func (c *Conn) ReadMessage() (int, []byte, error) {
	for {
		f, err := c.readFrame()
		if err != nil {
			return 0, nil, err
		}
		switch f.opcode {
		case OpClose:
			code := StatusNormalClosure
			if len(f.payload) >= 2 {
				code = int(binary.BigEndian.Uint16(f.payload[0:2]))
			}
			_ = c.writeClose(StatusNormalClosure, nil)
			return 0, nil, &CloseError{Code: code, Text: string(safeReason(f.payload))}
		case OpPing:
			if len(f.payload) > maxControlPayload {
				return 0, nil, protocolError("ping payload too large")
			}
			if err := c.writeControl(OpPong, f.payload); err != nil {
				return 0, nil, err
			}
			continue
		case OpPong:
			continue
		case OpText, OpBinary:
			if c.fragActive {
				return 0, nil, protocolError("new data frame before finishing fragmented message")
			}
			if isControl(f.opcode) {
				return 0, nil, protocolError("bad opcode")
			}
			c.fragActive = true
			c.fragOpcode = f.opcode
			c.fragBuf.Reset()
			if err := c.appendPayload(f.payload); err != nil {
				return 0, nil, err
			}
		case OpContinuation:
			if !c.fragActive {
				return 0, nil, protocolError("continuation without start frame")
			}
			if err := c.appendPayload(f.payload); err != nil {
				return 0, nil, err
			}
		default:
			return 0, nil, protocolError(fmt.Sprintf("unknown opcode %d", f.opcode))
		}
		if f.fin {
			if !f.isDataStart() && f.opcode != OpContinuation {
				return 0, nil, protocolError("expected final fragment")
			}
			opcode := c.fragOpcode
			data := make([]byte, c.fragBuf.Len())
			copy(data, c.fragBuf.Bytes())
			c.fragActive = false
			c.fragBuf.Reset()
			return int(opcode), data, nil
		}
	}
}

func (f *frameHeader) isDataStart() bool { return f.opcode == OpText || f.opcode == OpBinary }

func (c *Conn) appendPayload(p []byte) error {
	if int64(c.fragBuf.Len()+len(p)) > c.maxMessageSize {
		_ = c.writeClose(StatusMessageTooBig, []byte("message too big"))
		return &CloseError{Code: StatusMessageTooBig, Text: "message too big"}
	}
	_, err := c.fragBuf.Write(p)
	return err
}

func isControl(op byte) bool { return op >= OpClose && op <= OpPong }

func safeReason(p []byte) []byte {
	if len(p) <= 2 {
		return nil
	}
	return p[2:]
}

// WriteMessage sends a complete (single-frame) data message. Server frames are
// not masked per RFC 6455.
func (c *Conn) WriteMessage(opcode int, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed {
		return ErrCloseSent
	}
	return writeFrame(c.nc, false, byte(opcode), data)
}

func (c *Conn) writeControl(opcode byte, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed {
		return ErrCloseSent
	}
	if len(data) > maxControlPayload {
		return errors.New("control payload too large")
	}
	return writeFrame(c.nc, true, opcode, data)
}

func (c *Conn) writeClose(code int, reason []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed {
		return nil
	}
	payload := make([]byte, 2+len(reason))
	binary.BigEndian.PutUint16(payload[0:2], uint16(code))
	copy(payload[2:], reason)
	err := writeFrame(c.nc, true, OpClose, payload)
	c.closed = true
	return err
}

// Close sends a normal close frame (best effort) and closes the connection.
func (c *Conn) Close() error {
	var werr error
	c.closeOnce.Do(func() {
		werr = c.writeClose(StatusNormalClosure, nil)
	})
	if cerr := c.nc.Close(); cerr != nil && werr == nil {
		werr = cerr
	}
	return werr
}

// RemoteAddr returns the peer address.
func (c *Conn) RemoteAddr() net.Addr { return c.nc.RemoteAddr() }

func writeFrame(w io.Writer, finControl bool, opcode byte, payload []byte) error {
	var header [10]byte
	n := 0
	finBit := byte(0x80)
	if !finControl && (opcode == OpPing || opcode == OpPong || opcode == OpClose) {
		finBit = 0x80
	}
	header[0] = finBit | opcode
	l := len(payload)
	switch {
	case l <= smallPayload:
		header[1] = byte(l)
		n = 2
	case l <= 65535:
		header[1] = extended16
		binary.BigEndian.PutUint16(header[2:4], uint16(l))
		n = 4
	default:
		header[1] = extended64
		binary.BigEndian.PutUint64(header[2:10], uint64(l))
		n = 10
	}
	if _, err := w.Write(header[:n]); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// CloseError represents a WebSocket close.
type CloseError struct {
	Code int
	Text string
}

func (e *CloseError) Error() string { return fmt.Sprintf("ws: close %d: %s", e.Code, e.Text) }

func protocolError(msg string) error {
	return &CloseError{Code: StatusProtocolError, Text: msg}
}
