// Package relay implements the Nostr relay business logic: subscription
// management, event validation/persistence and real-time fan-out.
package relay

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gitstq/nostromo-relay/internal/protocol"
	"github.com/gitstq/nostromo-relay/internal/store"
	"github.com/gitstq/nostromo-relay/internal/ws"
)

// Config tunes relay behavior.
type Config struct {
	Name         string
	Description  string
	Contact      string
	PubKey       string
	DefaultLimit int
	MaxLimit     int
	RequireAuth  bool
	MaxMessageMB int64
}

// Defaults returns a sensible default configuration.
func Defaults() Config {
	return Config{
		Name:         "Nostromo",
		Description:  "A zero-dependency, single-binary Nostr event-log relay for developers and AI agents.",
		DefaultLimit: 100,
		MaxLimit:     1000,
		MaxMessageMB: 1,
	}
}

// Relay coordinates storage and live clients.
type Relay struct {
	store *store.Store
	cfg   Config

	mu      sync.RWMutex
	clients map[*Client]bool

	// metrics
	accepted atomicCount
	rejected atomicCount
}

// New constructs a relay over the given store.
func New(st *store.Store, cfg Config) *Relay {
	if cfg.DefaultLimit <= 0 {
		cfg = Defaults()
	}
	return &Relay{store: st, cfg: cfg, clients: map[*Client]bool{}}
}

// Store exposes the backing store.
func (r *Relay) Store() *store.Store { return r.store }

// Config returns the relay configuration.
func (r *Relay) Config() Config { return r.cfg }

// Stats returns simple counters and connected client count.
func (r *Relay) Stats() (events, clients, accepted, rejected int64) {
	r.mu.RLock()
	clients = int64(len(r.clients))
	r.mu.RUnlock()
	return int64(r.store.Count()), clients, r.accepted.get(), r.rejected.get()
}

// Client is one WebSocket connection and its subscriptions.
type Client struct {
	relay *Relay
	conn  *ws.Conn
	url   string // self relay URL (ws/wss://host)

	mu   sync.Mutex
	subs map[string][]protocol.Filter

	challenge     string
	authenticated bool
	authPubKey    string
}

func (r *Relay) register(c *Client) {
	r.mu.Lock()
	r.clients[c] = true
	r.mu.Unlock()
}

func (r *Relay) unregister(c *Client) {
	r.mu.Lock()
	delete(r.clients, c)
	r.mu.Unlock()
}

// Serve runs the read loop for an upgraded connection.
func (r *Relay) Serve(conn *ws.Conn, selfURL string) {
	c := &Client{relay: r, conn: conn, url: selfURL, subs: map[string][]protocol.Filter{}}
	r.register(c)
	defer func() {
		r.unregister(c)
		conn.Close()
	}()

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		msg, perr := protocol.ParseClientMessage(data)
		if perr != nil {
			c.send(protocol.EncodeNotice(perr.Error()))
			continue
		}
		switch m := msg.(type) {
		case *protocol.EventMessage:
			if m.Event.Kind == protocol.KindAuth {
				c.handleAuthEvent(&m.Event)
			} else {
				c.handleEvent(&m.Event)
			}
		case *protocol.ReqMessage:
			c.handleReq(m)
		case *protocol.CloseMessage:
			c.mu.Lock()
			delete(c.subs, m.SubID)
			c.mu.Unlock()
		case *protocol.AuthMessage:
			c.handleAuthEvent(&m.Event)
		}
	}
}

func (c *Client) handleEvent(e *protocol.Event) {
	now := time.Now().Unix()
	if err := e.Validate(); err != nil {
		c.relay.rejected.add(1)
		c.send(protocol.EncodeOK(e.ID, false, "invalid: "+err.Error()))
		return
	}
	if exp, ok := e.Expiration(); ok && exp <= now {
		c.relay.rejected.add(1)
		c.send(protocol.EncodeOK(e.ID, false, "error: event already expired"))
		return
	}

	ephemeral := protocol.IsEphemeral(e.Kind)
	if !ephemeral {
		changed, err := c.relay.store.Add(e, now)
		if err != nil {
			c.send(protocol.EncodeOK(e.ID, false, "error: storage failure"))
			return
		}
		if !changed {
			// Duplicate of an existing event/identical replacement: still OK.
			c.send(protocol.EncodeOK(e.ID, true, "duplicate:"))
			return
		}
	}
	c.relay.accepted.add(1)
	c.send(protocol.EncodeOK(e.ID, true, ""))
	c.relay.broadcast(e)
}

func (c *Client) handleReq(m *protocol.ReqMessage) {
	if c.relay.cfg.RequireAuth && !c.authenticated {
		c.send(protocol.EncodeAuthChallenge(c.newChallenge()))
		c.send(protocol.EncodeClosed(m.SubID, "auth-required: authentication is required"))
		return
	}
	// Clamp per-filter limits.
	filters := make([]protocol.Filter, len(m.Filters))
	copy(filters, m.Filters)
	for i := range filters {
		if filters[i].Limit > c.relay.cfg.MaxLimit {
			filters[i].Limit = c.relay.cfg.MaxLimit
		}
	}
	c.mu.Lock()
	c.subs[m.SubID] = filters
	c.mu.Unlock()

	now := time.Now().Unix()
	events := c.relay.store.Query(filters, now, c.relay.cfg.DefaultLimit)
	for _, e := range events {
		c.send(protocol.EncodeOutgoingEvent(m.SubID, e))
	}
	c.send(protocol.EncodeEOSE(m.SubID))
}

// broadcast fans an event out to every matching live subscription.
func (r *Relay) broadcast(e *protocol.Event) {
	r.mu.RLock()
	clients := make([]*Client, 0, len(r.clients))
	for c := range r.clients {
		clients = append(clients, c)
	}
	r.mu.RUnlock()

	for _, c := range clients {
		c.mu.Lock()
		subs := c.subs
		for subID, filters := range subs {
			for i := range filters {
				if filters[i].Match(e) {
					c.send(protocol.EncodeOutgoingEvent(subID, e))
					break
				}
			}
		}
		c.mu.Unlock()
	}
}

func (c *Client) send(b []byte) {
	_ = c.conn.WriteMessage(ws.OpText, b)
}

func (c *Client) newChallenge() string {
	if c.challenge == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return ""
		}
		c.challenge = hex.EncodeToString(b)
	}
	return c.challenge
}

// atomicCount is a simple int64 counter safe for concurrent use.
type atomicCount int64

func (a *atomicCount) add(n int64) { atomic.AddInt64((*int64)(a), n) }
func (a *atomicCount) get() int64  { return atomic.LoadInt64((*int64)(a)) }
