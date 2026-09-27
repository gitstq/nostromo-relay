package store

import (
	"math/big"
	"path/filepath"
	"testing"

	"github.com/gitstq/nostromo-relay/internal/crypto"
	"github.com/gitstq/nostromo-relay/internal/protocol"
)

func signed(t *testing.T, priv *big.Int, kind int, created int64, content string, tags protocol.Tags) *protocol.Event {
	t.Helper()
	e := &protocol.Event{CreatedAt: created, Kind: kind, Tags: tags, Content: content}
	if err := protocol.SignEvent(e, priv); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestAddReplaceableParam(t *testing.T) {
	s := New()
	priv, _ := crypto.GeneratePrivateKey()
	now := int64(1000)

	// Two metadata events: only the newer survives.
	s.Add(signed(t, priv, 0, 1, "v1", nil), now)
	s.Add(signed(t, priv, 0, 2, "v2", nil), now)
	if s.Count() != 1 {
		t.Fatalf("replaceable count=%d want 1", s.Count())
	}
	res := s.Query([]protocol.Filter{{Kinds: []int{0}}}, now, 100)
	if len(res) != 1 || res[0].Content != "v2" {
		t.Fatal("replaceable resolution wrong")
	}

	// Parameterized: distinct d tags kept separately, same d replaced.
	s.Add(signed(t, priv, 30001, 1, "a", protocol.Tags{{"d", "a"}}), now)
	s.Add(signed(t, priv, 30001, 2, "b", protocol.Tags{{"d", "b"}}), now)
	s.Add(signed(t, priv, 30001, 3, "a2", protocol.Tags{{"d", "a"}}), now)
	res = s.Query([]protocol.Filter{{Kinds: []int{30001}}}, now, 100)
	if len(res) != 2 {
		t.Fatalf("param count=%d want 2", len(res))
	}
}

func TestDeletionAndExpiration(t *testing.T) {
	s := New()
	priv, _ := crypto.GeneratePrivateKey()
	now := int64(1000)

	note := signed(t, priv, 1, 5, "note", nil)
	s.Add(note, now)
	del := signed(t, priv, 5, 6, "", protocol.Tags{{"e", note.ID}})
	s.Add(del, now)
	res := s.Query([]protocol.Filter{{Kinds: []int{1}}}, now, 100)
	if len(res) != 0 {
		t.Fatal("deleted note still present")
	}

	// Future expiration stored; past expiration rejected.
	fut := signed(t, priv, 1, 7, "future", protocol.Tags{{"expiration", "2000"}})
	if ok, _ := s.Add(fut, now); !ok {
		t.Fatal("future-expiring event should be stored")
	}
	past := signed(t, priv, 1, 8, "past", protocol.Tags{{"expiration", "500"}})
	if ok, _ := s.Add(past, now); ok {
		t.Fatal("already-expired event should not be stored")
	}
	// After expiry the future event disappears from queries.
	res = s.Query([]protocol.Filter{{Kinds: []int{1}}}, 3000, 100)
	if len(res) != 0 {
		t.Fatal("expired event still returned")
	}
}

func TestQueryOrderLimit(t *testing.T) {
	s := New()
	priv, _ := crypto.GeneratePrivateKey()
	for i := int64(1); i <= 5; i++ {
		s.Add(signed(t, priv, 1, i, "x", nil), 1000)
	}
	res := s.Query([]protocol.Filter{{Kinds: []int{1}, Limit: 3}}, 1000, 100)
	if len(res) != 3 || res[0].CreatedAt != 5 || res[2].CreatedAt != 3 {
		t.Fatalf("limit/order wrong: %v", res)
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	priv, _ := crypto.GeneratePrivateKey()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.Add(signed(t, priv, 1, 1, "note", nil), 1000)
	s.Add(signed(t, priv, 0, 1, "meta-v1", nil), 1000)
	s.Add(signed(t, priv, 0, 2, "meta-v2", nil), 1000)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	// 1 note + 1 latest metadata = 2 live events after replay.
	if s2.Count() != 2 {
		t.Fatalf("replayed count=%d want 2", s2.Count())
	}
	res := s2.Query([]protocol.Filter{{Kinds: []int{0}}}, 1000, 100)
	if len(res) != 1 || res[0].Content != "meta-v2" {
		t.Fatal("replaceable replay wrong")
	}
}
