package redisserver

import (
	"strconv"
	"sync"
	"time"
)

// kind identifies which Redis data type a key currently holds. Real Redis
// requires a key to hold exactly one type at a time and returns a WRONGTYPE
// error on a type mismatch; dataStore mirrors that behaviour.
type kind int

const (
	kindNone kind = iota
	kindString
	kindHash
	kindList
)

// entry is the value stored for a single key, tagged with its kind so
// cross-type operations (e.g. HGET on a string key) can be rejected.
type entry struct {
	kind      kind
	str       string
	hash      map[string]string
	list      []string
	expiresAt time.Time // zero value means "never expires"
}

func (e *entry) expired(now time.Time) bool {
	return !e.expiresAt.IsZero() && now.After(e.expiresAt)
}

// wrongTypeErr is returned when a command is applied to a key holding a
// different data type, mirroring Redis's "WRONGTYPE" error.
type wrongTypeErr struct{}

func (wrongTypeErr) Error() string {
	return "WRONGTYPE Operation against a key holding the wrong kind of value"
}

// dataStore is a minimal, thread-safe, in-memory implementation of the
// subset of Redis command semantics needed for realistic stateful mocking:
// strings (with TTL), counters, hashes, and lists. It intentionally only
// implements this bounded command set — anything else is left to the
// existing static-mock matcher.
type dataStore struct {
	mu   sync.Mutex
	data map[string]*entry
}

func newDataStore() *dataStore {
	return &dataStore{data: make(map[string]*entry)}
}

// getLocked returns the live, non-expired entry for key, lazily deleting it
// if its TTL has elapsed. Caller must hold s.mu.
func (s *dataStore) getLocked(key string) (*entry, bool) {
	e, ok := s.data[key]
	if !ok {
		return nil, false
	}
	if e.expired(time.Now()) {
		delete(s.data, key)
		return nil, false
	}
	return e, true
}

// Flush removes every key, used by FLUSHDB/FLUSHALL in stateful mode.
func (s *dataStore) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]*entry)
}

// ---------------------------------------------------------------------------
// Strings / counters / TTL
// ---------------------------------------------------------------------------

// Set stores value as a string, overwriting any previous value/type. ttl <= 0
// means no expiry.
func (s *dataStore) Set(key, value string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := &entry{kind: kindString, str: value}
	if ttl > 0 {
		e.expiresAt = time.Now().Add(ttl)
	}
	s.data[key] = e
}

// Get returns the string value for key. ok is false if the key is absent or
// expired; err is wrongTypeErr if the key holds a non-string value.
func (s *dataStore) Get(key string) (value string, ok bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found {
		return "", false, nil
	}
	if e.kind != kindString {
		return "", false, wrongTypeErr{}
	}
	return e.str, true, nil
}

// Del removes keys and returns how many were actually present.
func (s *dataStore) Del(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, k := range keys {
		if _, found := s.getLocked(k); found {
			delete(s.data, k)
			n++
		}
	}
	return n
}

// Exists returns how many of the given keys are present (and not expired).
func (s *dataStore) Exists(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, k := range keys {
		if _, found := s.getLocked(k); found {
			n++
		}
	}
	return n
}

// Expire sets a TTL (in seconds) on an existing key. Returns false if the key
// doesn't exist.
func (s *dataStore) Expire(key string, seconds int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found {
		return false
	}
	e.expiresAt = time.Now().Add(time.Duration(seconds) * time.Second)
	return true
}

// TTL returns the remaining seconds until expiry, -1 if the key exists but
// has no expiry, or -2 if the key doesn't exist — matching Redis's TTL
// command semantics.
func (s *dataStore) TTL(key string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found {
		return -2
	}
	if e.expiresAt.IsZero() {
		return -1
	}
	remaining := time.Until(e.expiresAt)
	if remaining < 0 {
		return -2
	}
	return int64(remaining.Round(time.Second).Seconds())
}

// Persist removes any TTL from key, returning true if a TTL was removed.
func (s *dataStore) Persist(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found || e.expiresAt.IsZero() {
		return false
	}
	e.expiresAt = time.Time{}
	return true
}

// IncrBy adds delta to the integer value at key (defaulting to 0 if absent),
// storing and returning the new value. err is wrongTypeErr for a non-string
// key, or a parse error if the existing value isn't an integer.
func (s *dataStore) IncrBy(key string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found {
		e = &entry{kind: kindString, str: "0"}
		s.data[key] = e
	}
	if e.kind != kindString {
		return 0, wrongTypeErr{}
	}
	n, err := strconv.ParseInt(e.str, 10, 64)
	if err != nil {
		return 0, err
	}
	n += delta
	e.str = strconv.FormatInt(n, 10)
	return n, nil
}

// Append concatenates value onto the existing string at key (or creates it),
// returning the new total length.
func (s *dataStore) Append(key, value string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found {
		e = &entry{kind: kindString}
		s.data[key] = e
	}
	if e.kind != kindString {
		return 0, wrongTypeErr{}
	}
	e.str += value
	return len(e.str), nil
}

// ---------------------------------------------------------------------------
// Hashes
// ---------------------------------------------------------------------------

// HSet sets the given field/value pairs on the hash at key, returning the
// number of fields that were newly created (not merely updated).
func (s *dataStore) HSet(key string, fields map[string]string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found {
		e = &entry{kind: kindHash, hash: map[string]string{}}
		s.data[key] = e
	}
	if e.kind != kindHash {
		return 0, wrongTypeErr{}
	}
	added := 0
	for f, v := range fields {
		if _, exists := e.hash[f]; !exists {
			added++
		}
		e.hash[f] = v
	}
	return added, nil
}

// HGet returns the value of field in the hash at key.
func (s *dataStore) HGet(key, field string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found {
		return "", false, nil
	}
	if e.kind != kindHash {
		return "", false, wrongTypeErr{}
	}
	v, ok := e.hash[field]
	return v, ok, nil
}

// HGetAll returns a copy of all field/value pairs in the hash at key.
func (s *dataStore) HGetAll(key string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found {
		return map[string]string{}, nil
	}
	if e.kind != kindHash {
		return nil, wrongTypeErr{}
	}
	out := make(map[string]string, len(e.hash))
	for k, v := range e.hash {
		out[k] = v
	}
	return out, nil
}

// HDel removes the given fields from the hash at key, returning how many
// were actually present.
func (s *dataStore) HDel(key string, fields ...string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found {
		return 0, nil
	}
	if e.kind != kindHash {
		return 0, wrongTypeErr{}
	}
	n := 0
	for _, f := range fields {
		if _, ok := e.hash[f]; ok {
			delete(e.hash, f)
			n++
		}
	}
	return n, nil
}

// HExists reports whether field exists in the hash at key.
func (s *dataStore) HExists(key, field string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found {
		return false, nil
	}
	if e.kind != kindHash {
		return false, wrongTypeErr{}
	}
	_, ok := e.hash[field]
	return ok, nil
}

// ---------------------------------------------------------------------------
// Lists
// ---------------------------------------------------------------------------

// push prepends (left=true) or appends (left=false) values to the list at
// key, returning the new length.
func (s *dataStore) push(key string, left bool, values []string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found {
		e = &entry{kind: kindList}
		s.data[key] = e
	}
	if e.kind != kindList {
		return 0, wrongTypeErr{}
	}
	if left {
		// LPUSH pushes each argument so the last argument ends up at the
		// head, matching Redis's documented LPUSH ordering.
		for _, v := range values {
			e.list = append([]string{v}, e.list...)
		}
	} else {
		e.list = append(e.list, values...)
	}
	return len(e.list), nil
}

// LPush pushes values onto the head of the list at key.
func (s *dataStore) LPush(key string, values ...string) (int, error) {
	return s.push(key, true, values)
}

// RPush pushes values onto the tail of the list at key.
func (s *dataStore) RPush(key string, values ...string) (int, error) {
	return s.push(key, false, values)
}

// LRange returns the elements of the list at key between start and stop
// (inclusive), supporting Redis's negative-index-from-end convention.
func (s *dataStore) LRange(key string, start, stop int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found {
		return []string{}, nil
	}
	if e.kind != kindList {
		return nil, wrongTypeErr{}
	}
	n := len(e.list)
	start = normalizeListIndex(start, n)
	stop = normalizeListIndex(stop, n)
	if start > stop || start >= n || n == 0 {
		return []string{}, nil
	}
	if stop >= n {
		stop = n - 1
	}
	if start < 0 {
		start = 0
	}
	out := make([]string, stop-start+1)
	copy(out, e.list[start:stop+1])
	return out, nil
}

// LLen returns the length of the list at key (0 if absent).
func (s *dataStore) LLen(key string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, found := s.getLocked(key)
	if !found {
		return 0, nil
	}
	if e.kind != kindList {
		return 0, wrongTypeErr{}
	}
	return len(e.list), nil
}

// normalizeListIndex converts a Redis-style index (negative counts from the
// end, e.g. -1 is the last element) into a non-negative slice index.
func normalizeListIndex(i, n int) int {
	if i < 0 {
		i += n
	}
	return i
}
