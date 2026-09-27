package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gitstq/nostromo-relay/internal/protocol"
)

// Open creates a store backed by an append-only JSONL file at path. Existing
// events are replayed to rebuild the in-memory index.
func Open(path string) (*Store, error) {
	s := New()
	s.path = path
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	s.file = f

	now := time.Now().Unix()
	reader := bufio.NewReader(f)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	loaded := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e protocol.Event
		if err := json.Unmarshal(line, &e); err != nil {
			// Skip a corrupt line rather than aborting startup.
			continue
		}
		s.mu.Lock()
		if s.apply(&e, now) {
			loaded++
		}
		s.mu.Unlock()
	}
	if err := scanner.Err(); err != nil {
		return s, fmt.Errorf("reading event log: %w", err)
	}
	return s, nil
}

func (s *Store) appendLine(e *protocol.Event) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if s.file == nil {
		return fmt.Errorf("store is closed")
	}
	_, err = s.file.Write(b)
	return err
}

// Compact rewrites the log so it contains only the current live events,
// discarding superseded/deleted/expired history.
func (s *Store) Compact() error {
	if s.path == "" {
		return nil
	}
	s.mu.RLock()
	live := make([]*protocol.Event, len(s.events))
	copy(live, s.events)
	s.mu.RUnlock()

	tmp := s.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, e := range live {
		b, err := json.Marshal(e)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := w.Write(append(b, '\n')); err != nil {
			f.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	s.fileMu.Lock()
	if s.file != nil {
		s.file.Close()
	}
	if err := os.Rename(tmp, s.path); err != nil {
		s.fileMu.Unlock()
		return err
	}
	nf, err := os.OpenFile(s.path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		s.fileMu.Unlock()
		return err
	}
	s.file = nf
	s.fileMu.Unlock()
	return nil
}

// Close flushes and closes the backing file.
func (s *Store) Close() error {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if s.file != nil {
		err := s.file.Close()
		s.file = nil
		return err
	}
	return nil
}
