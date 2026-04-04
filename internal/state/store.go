// Package state provides a thread-safe in-memory key-value store used to drive
// stateful mock behaviour (e.g. "after POST /login the GET /me mock fires").
package state

import "sync"

// Store is a thread-safe string key-value map.
type Store struct {
	mu   sync.RWMutex
	data map[string]string
}

// New returns an initialised Store.
func New() *Store {
	return &Store{data: make(map[string]string)}
}

// Set writes a value.
func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// Get reads a value; returns ("", false) when absent.
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

// Delete removes a key.
func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

// All returns a snapshot of all entries.
func (s *Store) All() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	return out
}

// Reset removes all entries.
func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]string)
}
