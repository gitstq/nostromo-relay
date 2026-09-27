// Package protocol implements the Nostr data model: events, filters and the
// client/relay message vocabulary, independent of any transport.
package protocol

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/gitstq/nostromo-relay/internal/crypto"
)

// Tags is a Nostr event tag array.
type Tags [][]string

// Event is a signed Nostr event (NIP-01).
type Event struct {
	ID        string `json:"id"`
	PubKey    string `json:"pubkey"`
	CreatedAt int64  `json:"created_at"`
	Kind      int    `json:"kind"`
	Tags      Tags   `json:"tags"`
	Content   string `json:"content"`
	Sig       string `json:"sig"`
}

// well-known event kinds.
const (
	KindSetMetadata      = 0
	KindTextNote         = 1
	KindRecommendRelay   = 2
	KindContacts         = 3
	KindDeletion         = 5
	KindRepost           = 6
	KindChannelCreation  = 40
	KindChannelMessage   = 42
	KindGiftWrap         = 1059
	KindFileMetadata     = 1063
	KindAuth             = 22242
	KindParameterizedMin = 30000
	KindParameterizedMax = 39999
)

// Serialize returns the canonical NIP-01 serialization used to derive the id:
// [0, pubkey, created_at, kind, tags, content].
func (e *Event) Serialize() []byte {
	tags := e.Tags
	if tags == nil {
		tags = Tags{}
	}
	arr := []interface{}{0, e.PubKey, e.CreatedAt, e.Kind, tags, e.Content}
	b, err := json.Marshal(arr)
	if err != nil {
		// All constituent types are JSON-safe; this should never happen.
		panic(fmt.Sprintf("protocol: failed to serialize event: %v", err))
	}
	return b
}

// GetID computes the event id from its canonical serialization.
func (e *Event) GetID() string {
	sum := sha256.Sum256(e.Serialize())
	return hex.EncodeToString(sum[:])
}

// CheckID reports whether the stored id matches the canonical serialization.
func (e *Event) CheckID() bool {
	return e.ID == e.GetID()
}

// VerifySignature validates the Schnorr signature against the event id.
func (e *Event) VerifySignature() bool {
	pub, err := hex.DecodeString(e.PubKey)
	if err != nil || len(pub) != 32 {
		return false
	}
	id, err := hex.DecodeString(e.ID)
	if err != nil || len(id) != 32 {
		return false
	}
	sig, err := hex.DecodeString(e.Sig)
	if err != nil || len(sig) != 64 {
		return false
	}
	return crypto.SchnorrVerify(pub, id, sig)
}

// Validate performs full structural and cryptographic validation.
func (e *Event) Validate() error {
	if len(e.ID) != 64 || !isLowerHex(e.ID) {
		return fmt.Errorf("invalid id")
	}
	if len(e.PubKey) != 64 || !isLowerHex(e.PubKey) {
		return fmt.Errorf("invalid pubkey")
	}
	if len(e.Sig) != 128 || !isLowerHex(e.Sig) {
		return fmt.Errorf("invalid signature")
	}
	if e.CreatedAt < 0 {
		return fmt.Errorf("invalid created_at")
	}
	for i, tag := range e.Tags {
		if len(tag) == 0 {
			return fmt.Errorf("tag %d is empty", i)
		}
	}
	if !e.CheckID() {
		return fmt.Errorf("event id does not match serialization")
	}
	if !e.VerifySignature() {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

// SignEvent fills in the pubkey, id and signature for an event using priv.
func SignEvent(e *Event, priv *big.Int) error {
	e.PubKey = hex.EncodeToString(crypto.XOnlyPublicKey(priv))
	if e.Tags == nil {
		e.Tags = Tags{}
	}
	e.ID = e.GetID()
	idBytes, err := hex.DecodeString(e.ID)
	if err != nil {
		return err
	}
	aux := make([]byte, 32)
	if _, err := rand.Read(aux); err != nil {
		return err
	}
	sig, err := crypto.SchnorrSign(priv, idBytes, aux)
	if err != nil {
		return err
	}
	e.Sig = hex.EncodeToString(sig)
	return nil
}

func isLowerHex(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

// Values returns the second element of every tag with the given name.
func (e *Event) Values(name string) []string {
	var out []string
	for _, tag := range e.Tags {
		if len(tag) >= 2 && tag[0] == name {
			out = append(out, tag[1])
		}
	}
	return out
}

// DTag returns the NIP-33 "d" tag value (empty string if absent).
func (e *Event) DTag() string {
	for _, tag := range e.Tags {
		if len(tag) >= 2 && tag[0] == "d" {
			return tag[1]
		}
	}
	return ""
}

// Expiration returns the NIP-40 expiration timestamp, if present and valid.
func (e *Event) Expiration() (int64, bool) {
	for _, tag := range e.Tags {
		if len(tag) >= 2 && tag[0] == "expiration" {
			v, err := strconv.ParseInt(tag[1], 10, 64)
			if err != nil {
				return 0, false
			}
			return v, true
		}
	}
	return 0, false
}

// IsReplaceable reports whether the kind is a NIP-16 replaceable event.
func IsReplaceable(kind int) bool {
	return kind == KindSetMetadata || kind == KindContacts ||
		(kind >= 10000 && kind < 20000)
}

// IsEphemeral reports whether the kind is a NIP-16 ephemeral event.
func IsEphemeral(kind int) bool { return kind >= 20000 && kind < 30000 }

// IsParameterizedReplaceable reports a NIP-33 parameterized replaceable kind.
func IsParameterizedReplaceable(kind int) bool {
	return kind >= KindParameterizedMin && kind <= KindParameterizedMax
}

// ReplaceKey returns the dedup key for replaceable/parameterized events.
// It returns "" for regular events.
func (e *Event) ReplaceKey() string {
	switch {
	case IsParameterizedReplaceable(e.Kind):
		return fmt.Sprintf("%s:%d:%s", e.PubKey, e.Kind, e.DTag())
	case IsReplaceable(e.Kind):
		return fmt.Sprintf("%s:%d", e.PubKey, e.Kind)
	default:
		return ""
	}
}
