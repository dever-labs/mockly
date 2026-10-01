// Package state provides a thread-safe in-memory key-value store used to drive
// stateful mock behaviour (e.g. "after POST /login the GET /me mock fires").
package state

import (
	"strings"
	"sync"
	"time"
)

// entry pairs a stored value with its optional expiry. A zero ExpiresAt
// means the entry never expires.
type entry struct {
	value     string
	expiresAt time.Time
}

// expired reports whether the entry's TTL (if any) has elapsed as of now.
func (e entry) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && now.After(e.expiresAt)
}

// Store is a thread-safe string key-value map with optional per-key TTL
// expiry. Expiry is checked lazily (on Get/All) rather than via a sweep
// goroutine, so an expired key simply disappears the next time it's read.
type Store struct {
	mu   sync.RWMutex
	data map[string]entry
}

// New returns an initialised Store.
func New() *Store {
	return &Store{data: make(map[string]entry)}
}

// Set writes a value with no expiry. Equivalent to SetTTL(key, value, 0).
func (s *Store) Set(key, value string) {
	s.SetTTL(key, value, 0)
}

// SetTTL writes a value that expires after ttl elapses. ttl <= 0 means the
// value never expires (the same behaviour as Set).
func (s *Store) SetTTL(key, value string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := entry{value: value}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	}
	s.data[key] = e
}

// Get reads a value; returns ("", false) when absent or expired. An expired
// key is lazily removed from the store.
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	e, ok := s.data[key]
	s.mu.RUnlock()
	if !ok {
		return "", false
	}
	if e.expired(time.Now()) {
		s.Delete(key)
		return "", false
	}
	return e.value, true
}

// Delete removes a key.
func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

// All returns a snapshot of all non-expired entries, lazily dropping any
// expired keys encountered along the way.
func (s *Store) All() map[string]string {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.data))
	for k, e := range s.data {
		if e.expired(now) {
			delete(s.data, k)
			continue
		}
		out[k] = e.value
	}
	return out
}

// Reset removes all entries.
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]entry)
}

// ResetPrefix removes only the entries whose key starts with prefix, leaving
// all other state untouched. This lets independent mocks/scenarios that
// namespace their keys (e.g. "login:session") clear their own state without
// clobbering state belonging to other concurrently running mocks/tests. An
// empty prefix matches every key, behaving like Reset.
func (s *Store) ResetPrefix(prefix string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.data {
		if strings.HasPrefix(k, prefix) {
			delete(s.data, k)
		}
	}
}
