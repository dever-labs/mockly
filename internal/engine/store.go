package engine

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/dever-labs/mockly/internal/state"
)

// sharedStore holds the single *state.Store instance wired in by SetStore,
// so the "state" and "state_set" template functions can read/write the same
// runtime state store used for "state:" match conditions and the
// PUT/GET/DELETE /api/state management endpoints — without threading a
// *state.Store through every Render*/BuildFuncMap call site.
//
// There is exactly one *state.Store per running mockly process (created once
// in cmd/mockly and shared across every protocol server and the management
// API), so a single package-level pointer here is safe and mirrors existing
// package-level globals in this package (e.g. rng, seqCounters).
var sharedStore atomic.Pointer[state.Store]

// SetStore wires the shared runtime state store into the engine package.
// Call this once at startup with the same *state.Store passed to every
// protocol server, before any requests are served.
func SetStore(s *state.Store) {
	sharedStore.Store(s)
}

// getStore returns the shared state store, lazily creating a private
// fallback instance if SetStore was never called (e.g. in unit tests that
// exercise BuildFuncMap directly without a running server).
func getStore() *state.Store {
	if s := sharedStore.Load(); s != nil {
		return s
	}
	s := state.New()
	sharedStore.CompareAndSwap(nil, s)
	return sharedStore.Load()
}

// fnStateGet returns the value stored under key, or "" if absent/expired.
func fnStateGet(key string) string {
	v, _ := getStore().Get(key)
	return v
}

// fnStateSet writes value under key with no expiry and returns "" (empty
// string), so it can be used in a template purely for its side effect, e.g.
// {{- state_set "login:nonce" .query.nonce -}}.
func fnStateSet(key, value string) string {
	getStore().Set(key, value)
	return ""
}

// fnStateSetTTL writes value under key, expiring after ttlSeconds — e.g. for
// short-lived values such as an OAuth2 authorization code's associated
// nonce, which only needs to survive until the matching /token exchange.
func fnStateSetTTL(key, value string, ttlSeconds int) (string, error) {
	if ttlSeconds <= 0 {
		return "", fmt.Errorf("state_set_ttl: ttlSeconds must be > 0, got %d", ttlSeconds)
	}
	getStore().SetTTL(key, value, time.Duration(ttlSeconds)*time.Second)
	return "", nil
}
