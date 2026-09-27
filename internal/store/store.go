// Package store provides the relay's event storage: an in-memory index with
// optional append-only JSONL persistence (the "event log" core).
package store

import (
	"os"
	"sort"
	"sync"

	"github.com/gitstq/nostromo-relay/internal/protocol"
)

// Store is an in-memory event index, optionally backed by an append-only log.
type Store struct {
	mu sync.RWMutex

	events []*protocol.Event // current (live) events
	byID   map[string]*protocol.Event
	rk     map[string]string // replace key -> event id

	path   string // "" = memory only
	file   *os.File
	fileMu sync.Mutex
}

// New creates a memory-only store.
func New() *Store {
	return &Store{byID: map[string]*protocol.Event{}, rk: map[string]string{}}
}

// apply inserts or replaces an event without persisting. It returns whether
// the live set changed. now is the current unix time used for expiration.
func (s *Store) apply(e *protocol.Event, now int64) bool {
	if exp, ok := e.Expiration(); ok && exp <= now {
		return false
	}

	// NIP-09 deletion: remove same-author events listed in "e" tags.
	if e.Kind == protocol.KindDeletion {
		s.deleteRefs(e.Values("e"), e.PubKey)
	}

	key := e.ReplaceKey()
	if key != "" {
		if oldID, ok := s.rk[key]; ok {
			if oldID == e.ID {
				return false // identical replacement, no change
			}
			s.removeLocked(oldID)
		}
		s.rk[key] = e.ID
	} else if _, dup := s.byID[e.ID]; dup {
		return false
	}

	s.events = append(s.events, e)
	s.byID[e.ID] = e
	return true
}

// Add applies an event and (for a persistent store) appends it to the log.
func (s *Store) Add(e *protocol.Event, now int64) (bool, error) {
	s.mu.Lock()
	changed := s.apply(e, now)
	s.mu.Unlock()
	if !changed {
		return false, nil
	}
	if s.path != "" {
		if err := s.appendLine(e); err != nil {
			return true, err
		}
	}
	return true, nil
}

// deleteRefs removes same-author events with the given ids (NIP-09).
func (s *Store) deleteRefs(ids []string, pubkey string) int {
	n := 0
	for _, id := range ids {
		if ev, ok := s.byID[id]; ok && ev.PubKey == pubkey {
			s.removeLocked(id)
			n++
		}
	}
	return n
}

func (s *Store) removeLocked(id string) {
	ev, ok := s.byID[id]
	if !ok {
		return
	}
	for i, x := range s.events {
		if x.ID == id {
			s.events = append(s.events[:i], s.events[i+1:]...)
			break
		}
	}
	delete(s.byID, id)
	if key := ev.ReplaceKey(); key != "" {
		if s.rk[key] == id {
			delete(s.rk, key)
		}
	}
}

// Count returns the number of live events.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

// Query evaluates a set of OR-ed filters, dedupes and returns events ordered
// by created_at descending, excluding expired events.
func (s *Store) Query(filters []protocol.Filter, now int64, defLimit int) []*protocol.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := map[string]bool{}
	var out []*protocol.Event
	for i := range filters {
		f := &filters[i]
		limit := f.EffectiveLimit(defLimit)
		var matched []*protocol.Event
		for _, e := range s.events {
			if exp, ok := e.Expiration(); ok && exp <= now {
				continue
			}
			if f.Match(e) {
				matched = append(matched, e)
			}
		}
		sort.Slice(matched, func(a, b int) bool {
			if matched[a].CreatedAt != matched[b].CreatedAt {
				return matched[a].CreatedAt > matched[b].CreatedAt
			}
			return matched[a].ID > matched[b].ID
		})
		if len(matched) > limit {
			matched = matched[:limit]
		}
		for _, e := range matched {
			if !seen[e.ID] {
				seen[e.ID] = true
				out = append(out, e)
			}
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].CreatedAt != out[b].CreatedAt {
			return out[a].CreatedAt > out[b].CreatedAt
		}
		return out[a].ID > out[b].ID
	})
	return out
}
