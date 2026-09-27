package protocol

import (
	"testing"

	"github.com/gitstq/nostromo-relay/internal/crypto"
)

// TestNIP01EventID uses the canonical NIP-01 example event (content "测试中文")
// to verify serialization and id derivation.
func TestNIP01EventID(t *testing.T) {
	e := Event{
		PubKey:    "5aa5e90e0b16dba8cee4269e4e96aa2c33f3a8b71dc9f23ecbe3c7e1840845fe",
		CreatedAt: 1610865571,
		Kind:      KindTextNote,
		Tags:      Tags{},
		Content:   "测试中文",
	}
	// Expected id cross-verified with an independent Python implementation of
	// the NIP-01 canonical serialization.
	const want = "63b95f8163109989284726505fa73dc878540c9f8c2865436682ac27bc79e970"
	if got := e.GetID(); got != want {
		t.Fatalf("id mismatch\n got %s\nwant %s", got, want)
	}
}

func TestSignAndValidate(t *testing.T) {
	for i := 0; i < 10; i++ {
		priv, err := crypto.GeneratePrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		e := Event{
			CreatedAt: 1700000000 + int64(i),
			Kind:      KindTextNote,
			Tags:      Tags{{"t", "nostromo"}, {"expiration", "4102444800"}},
			Content:   "hello nostromo",
		}
		if err := SignEvent(&e, priv); err != nil {
			t.Fatal(err)
		}
		if err := e.Validate(); err != nil {
			t.Fatalf("valid event rejected: %v", err)
		}
		// Tamper with content.
		bad := e
		bad.Content = "tampered"
		if bad.CheckID() {
			t.Fatal("tampered event kept id valid")
		}
	}
}

func TestKindClassification(t *testing.T) {
	if !IsReplaceable(0) || !IsReplaceable(3) || !IsReplaceable(10000) || IsReplaceable(1) {
		t.Fatal("replaceable classification wrong")
	}
	if !IsEphemeral(20001) || IsEphemeral(19999) {
		t.Fatal("ephemeral classification wrong")
	}
	if !IsParameterizedReplaceable(30001) || IsParameterizedReplaceable(29999) {
		t.Fatal("parameterized classification wrong")
	}
}

func TestReplaceKey(t *testing.T) {
	e := Event{PubKey: "pk", Kind: 30001, Tags: Tags{{"d", "profile"}}}
	if got, want := e.ReplaceKey(), "pk:30001:profile"; got != want {
		t.Fatalf("param key %q want %q", got, want)
	}
	r := Event{PubKey: "pk", Kind: 0}
	if got, want := r.ReplaceKey(), "pk:0"; got != want {
		t.Fatalf("replaceable key %q want %q", got, want)
	}
	regular := Event{PubKey: "pk", Kind: 1}
	if regular.ReplaceKey() != "" {
		t.Fatal("regular event should have no replace key")
	}
}

func TestExpiration(t *testing.T) {
	e := Event{Tags: Tags{{"expiration", "1700000000"}}}
	v, ok := e.Expiration()
	if !ok || v != 1700000000 {
		t.Fatal("expiration parse failed")
	}
}
