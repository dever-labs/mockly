// Package scenarios manages test scenarios and fault injection.
package scenarios

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/dever-labs/mockly/internal/config"
)

// Store holds scenario definitions, tracks which are active, and manages the
// direct fault configuration used for chaos/latency injection.
type Store struct {
	mu          sync.RWMutex
	scenarios   map[string]config.Scenario
	active      map[string]bool
	direct      config.ProtocolFaults
	rng         *rand.Rand
	rateWindows map[string]*rateWindow // rate_limit fault sliding window, keyed by fault owner
}

// rateWindow tracks how many requests have been observed in the current
// trailing one-second window for a rate_limit fault.
type rateWindow struct {
	start time.Time
	count int
}

// New creates a Store pre-loaded with the given scenario definitions.
func New(initial []config.Scenario) *Store {
	s := &Store{
		scenarios:   make(map[string]config.Scenario),
		active:      make(map[string]bool),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())), // #nosec G404 -- error rate RNG does not need crypto randomness
		rateWindows: make(map[string]*rateWindow),
	}
	for _, sc := range initial {
		if sc.ID == "" {
			sc.ID = fmt.Sprintf("scenario-%d", time.Now().UnixNano())
		}
		s.scenarios[sc.ID] = sc
	}
	return s
}

// All returns all defined scenarios sorted by ID.
func (s *Store) All() []config.Scenario {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]config.Scenario, 0, len(s.scenarios))
	for _, sc := range s.scenarios {
		out = append(out, sc)
	}
	return out
}

// Get returns a scenario by ID.
func (s *Store) Get(id string) (config.Scenario, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sc, ok := s.scenarios[id]
	return sc, ok
}

// Set creates or replaces a scenario. Auto-assigns an ID if empty.
func (s *Store) Set(sc config.Scenario) config.Scenario {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sc.ID == "" {
		sc.ID = fmt.Sprintf("scenario-%d", time.Now().UnixNano())
	}
	s.scenarios[sc.ID] = sc
	return sc
}

// Delete removes a scenario (and deactivates it). Returns false if not found.
func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.scenarios[id]
	delete(s.scenarios, id)
	delete(s.active, id)
	return ok
}

// Activate marks a scenario as active. Returns false if the scenario doesn't exist.
func (s *Store) Activate(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.scenarios[id]; !ok {
		return false
	}
	s.active[id] = true
	return true
}

// Deactivate removes a scenario from the active set.
func (s *Store) Deactivate(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.active, id)
}

// ActiveIDs returns the IDs of all currently active scenarios.
func (s *Store) ActiveIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.active))
	for id := range s.active {
		ids = append(ids, id)
	}
	return ids
}

// ActiveScenarios returns all active scenario objects.
func (s *Store) ActiveScenarios() []config.Scenario {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]config.Scenario, 0, len(s.active))
	for id := range s.active {
		if sc, ok := s.scenarios[id]; ok {
			out = append(out, sc)
		}
	}
	return out
}

// PatchFor returns the first active scenario patch that targets mockID, or nil.
// If multiple active scenarios patch the same mock, the first found wins.
func (s *Store) PatchFor(mockID string) *config.MockPatch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for id := range s.active {
		sc, ok := s.scenarios[id]
		if !ok {
			continue
		}
		for i := range sc.Patches {
			if sc.Patches[i].MockID == mockID {
				cp := sc.Patches[i]
				return &cp
			}
		}
	}
	return nil
}

// SetDirectFaults replaces all direct protocol faults.
func (s *Store) SetDirectFaults(f config.ProtocolFaults) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.direct = f
}

// DirectFaults returns the current direct (non-scenario) fault config.
func (s *Store) DirectFaults() config.ProtocolFaults {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.direct
}

// ClearDirectFaults removes all direct faults.
func (s *Store) ClearDirectFaults() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.direct = config.ProtocolFaults{}
}

// RollFault returns true if a fault should apply to this particular request,
// based on the configured error_rate probability. A rate of 0 (or unset) means
// always apply; 1.0 also always applies.
func (s *Store) RollFault(rate float64) bool {
	if rate <= 0 || rate >= 1.0 {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.Float64() < rate
}

// ShouldFault returns true if a fault with the given error rate should apply.
func (s *Store) ShouldFault(rate float64) bool {
	return s.RollFault(rate)
}

// ResolveDelay picks the delay to apply for a fault/mock. If rng is non-nil
// and specifies a valid range (Max > 0), a uniform-random duration in
// [Min, Max] is returned (Min/Max swapped if given in the wrong order);
// otherwise the fixed duration is returned unchanged. This lets a single
// fixed delay (e.g. delay: 200ms) keep working exactly as before, while
// delay_range simulates jittery real-world latency instead of one constant
// value.
func (s *Store) ResolveDelay(fixed time.Duration, rng *config.DelayRange) time.Duration {
	if rng == nil || rng.Max.Duration <= 0 {
		return fixed
	}
	minD, maxD := rng.Min.Duration, rng.Max.Duration
	if maxD < minD {
		minD, maxD = maxD, minD
	}
	if maxD == minD {
		return minD
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return minD + time.Duration(s.rng.Int63n(int64(maxD-minD)))
}

// RateLimited reports whether a rate_limit fault should trigger for key
// (typically "http:global" for the direct/global fault, or
// "http:mock:<id>" for a per-mock fault), based on requests observed in the
// trailing one-second window. A requestsPerSecond of 0 or less disables
// rate limiting (always returns false).
func (s *Store) RateLimited(key string, requestsPerSecond int) bool {
	if requestsPerSecond <= 0 {
		return false
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	w, ok := s.rateWindows[key]
	if !ok || now.Sub(w.start) >= time.Second {
		w = &rateWindow{start: now}
		s.rateWindows[key] = w
	}
	w.count++
	return w.count > requestsPerSecond
}
