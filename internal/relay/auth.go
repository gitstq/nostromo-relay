package relay

import (
	"time"

	"github.com/gitstq/nostromo-relay/internal/protocol"
)

const authWindowSeconds = 600

// RequestAuth sends an AUTH challenge to the client (NIP-42).
func (c *Client) RequestAuth() {
	ch := c.newChallenge()
	if ch != "" {
		c.send(protocol.EncodeAuthChallenge(ch))
	}
}

// handleAuthEvent validates a kind-22242 authentication event.
func (c *Client) handleAuthEvent(e *protocol.Event) {
	fail := func(reason string) {
		c.send(protocol.EncodeOK(e.ID, false, "auth-required: "+reason))
	}
	if e.Kind != protocol.KindAuth {
		fail("not an auth event")
		return
	}
	if c.challenge == "" {
		// Client authenticated before being challenged; issue one and ask retry.
		c.RequestAuth()
		fail("no active challenge")
		return
	}
	if err := e.Validate(); err != nil {
		fail(err.Error())
		return
	}
	challenge := ""
	relay := ""
	for _, tag := range e.Tags {
		switch {
		case len(tag) >= 2 && tag[0] == "challenge":
			challenge = tag[1]
		case len(tag) >= 2 && tag[0] == "relay":
			relay = tag[1]
		}
	}
	if challenge == "" || challenge != c.challenge {
		fail("challenge mismatch")
		return
	}
	if relay == "" {
		fail("missing relay tag")
		return
	}
	now := time.Now().Unix()
	if e.CreatedAt > now+authWindowSeconds || e.CreatedAt < now-authWindowSeconds {
		fail("auth event timestamp out of range")
		return
	}

	c.authenticated = true
	c.authPubKey = e.PubKey
	// Challenge is single-use.
	c.challenge = ""
	c.send(protocol.EncodeOK(e.ID, true, ""))
}

// Authenticated reports whether the client completed NIP-42 auth.
func (c *Client) Authenticated() bool { return c.authenticated }
