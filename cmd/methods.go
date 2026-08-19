package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Store wraps a JSON file on disk with an in-process mutex. Open it once
// per command invocation, mutate, and it saves on every write — there's
// no separate explicit Save() the caller needs to remember to call.
type Store struct {
	path string
	mu   sync.Mutex
	d    data
}

// OpenDefault opens the store used by all CLI commands.
func OpenDefault() (*Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return Open(path)
}

// Open loads the store from path, creating an empty one if the file
// doesn't exist yet.
func Open(path string) (*Store, error) {
	s := &Store{path: path, d: emptyData()}

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil // fresh store, nothing to load
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if err := json.Unmarshal(raw, &s.d); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	// Guard against nil maps if the file predates a field being added.
	if s.d.Paused == nil {
		s.d.Paused = make(map[string]bool)
	}
	if s.d.Priority == nil {
		s.d.Priority = make(map[string]int)
	}
	if s.d.Stats == nil {
		s.d.Stats = make(map[string]map[string]int)
	}
	return s, nil
}

// save writes the store atomically: write to a temp file, then rename
// over the real path, so a crash mid-write never leaves a corrupt file.
func (s *Store) save() error {
	raw, err := json.MarshalIndent(s.d, "", "  ")
	if err != nil {
		return fmt.Errorf("encode store: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("replace store file: %w", err)
	}
	return nil
}

func scopeKey(provider, model string) string {
	if model == "" {
		return provider
	}
	return provider + "/" + model
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------- Keys ----------

func (s *Store) AddKey(provider, key, label string) (KeyRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := KeyRecord{
		ID:       newID(),
		Provider: provider,
		Key:      key,
		Label:    label,
		AddedAt:  time.Now(),
	}
	s.d.Keys = append(s.d.Keys, rec)

	if err := s.save(); err != nil {
		return KeyRecord{}, err
	}
	return rec, nil
}

// ListKeys returns keys, optionally filtered by provider ("" = all).
func (s *Store) ListKeys(provider string) []KeyRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	if provider == "" {
		out := make([]KeyRecord, len(s.d.Keys))
		copy(out, s.d.Keys)
		return out
	}

	var out []KeyRecord
	for _, k := range s.d.Keys {
		if k.Provider == provider {
			out = append(out, k)
		}
	}
	return out
}

// ---------- Pause / resume ----------

func (s *Store) SetPaused(provider, model string, paused bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := scopeKey(provider, model)
	if paused {
		s.d.Paused[key] = true
	} else {
		delete(s.d.Paused, key) // absence means "not paused" — keeps the file tidy
	}
	return s.save()
}

// IsPaused checks both the specific model and the whole-provider pause,
// since pausing a provider should silence every model under it.
func (s *Store) IsPaused(provider, model string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.d.Paused[provider] {
		return true
	}
	return s.d.Paused[scopeKey(provider, model)]
}

// ---------- Priority ----------

func (s *Store) SetPriority(provider, model string, priority int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.d.Priority[scopeKey(provider, model)] = priority
	return s.save()
}

// Priority returns the configured priority for a model, defaulting to 0
// (highest preference) if it was never explicitly set.
func (s *Store) Priority(provider, model string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.d.Priority[scopeKey(provider, model)]
}

// ---------- Stats ----------

// RecordRequest increments today's counter for a provider/model. The
// router should call this once per completed request.
func (s *Store) RecordRequest(provider, model string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if s.d.Stats[today] == nil {
		s.d.Stats[today] = make(map[string]int)
	}
	s.d.Stats[today][scopeKey(provider, model)]++
	return s.save()
}

// StatKind selects how StatsFor aggregates.
type StatKind string

const (
	StatsDay   StatKind = "day"
	StatsMonth StatKind = "month"
)

// StatsFor returns request counts keyed by "provider/model", summed over
// today (StatsDay) or the current calendar month (StatsMonth).
// Optionally filtered to one provider.
func (s *Store) StatsFor(kind StatKind, provider string) map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	out := make(map[string]int)

	for dateStr, counts := range s.d.Stats {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue // skip malformed entries rather than fail the whole read
		}

		matches := false
		switch kind {
		case StatsDay:
			matches = date.Format("2006-01-02") == now.Format("2006-01-02")
		case StatsMonth:
			matches = date.Year() == now.Year() && date.Month() == now.Month()
		}
		if !matches {
			continue
		}

		for scope, count := range counts {
			if provider != "" {
				// scope is "provider" or "provider/model" — only keep
				// entries belonging to the requested provider.
				if scope != provider && !hasPrefix(scope, provider+"/") {
					continue
				}
			}
			out[scope] += count
		}
	}
	return out
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// Path returns the file path this store is backed by, mainly for CLI
// output like "stored at ~/.hydra/store.json".
func (s *Store) Path() string {
	return s.path
}
