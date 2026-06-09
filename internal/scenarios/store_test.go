package scenarios_test

import (
	"testing"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/scenarios"
)
func TestStore_SetAndGet(t *testing.T) {
	s := scenarios.New(nil)

	sc := config.Scenario{ID: "sc1", Name: "Error mode", Patches: []config.MockPatch{
		{MockID: "m1", Status: 503},
	}}
	got := s.Set(sc)
	if got.ID != "sc1" {
		t.Fatalf("want id sc1, got %q", got.ID)
	}

	retrieved, ok := s.Get("sc1")
	if !ok {
		t.Fatal("expected to find sc1")
	}
	if retrieved.Name != "Error mode" {
		t.Errorf("unexpected name %q", retrieved.Name)
	}

	_, ok2 := s.Get("nonexistent")
	if ok2 {
		t.Fatal("expected not found for unknown ID")
	}
}

func TestStore_SetAutoID(t *testing.T) {
	s := scenarios.New(nil)
	sc := s.Set(config.Scenario{Name: "no-id"})
	if sc.ID == "" {
		t.Fatal("expected auto-generated ID")
	}
}

func TestStore_LoadedFromInitial(t *testing.T) {
	initial := []config.Scenario{
		{ID: "s1", Name: "One"},
		{ID: "s2", Name: "Two"},
	}
	s := scenarios.New(initial)
	all := s.All()
	if len(all) != 2 {
		t.Fatalf("want 2 scenarios, got %d", len(all))
	}
}

func TestStore_Delete(t *testing.T) {
	s := scenarios.New([]config.Scenario{{ID: "s1"}})
	s.Activate("s1")

	deleted := s.Delete("s1")
	if !deleted {
		t.Fatal("expected deleted=true")
	}
	if s.Delete("s1") {
		t.Fatal("second delete should return false")
	}
	if len(s.ActiveIDs()) != 0 {
		t.Fatal("deleted scenario should be removed from active set")
	}
}

func TestStore_ActivateDeactivate(t *testing.T) {
	s := scenarios.New([]config.Scenario{{ID: "s1"}})

	if s.Activate("nonexistent") {
		t.Fatal("activating nonexistent should return false")
	}

	if !s.Activate("s1") {
		t.Fatal("activating existing should return true")
	}
	ids := s.ActiveIDs()
	if len(ids) != 1 || ids[0] != "s1" {
		t.Fatalf("expected [s1] active, got %v", ids)
	}

	s.Deactivate("s1")
	if len(s.ActiveIDs()) != 0 {
		t.Fatal("expected no active scenarios after deactivate")
	}
}

func TestStore_ActiveScenarios(t *testing.T) {
	s := scenarios.New([]config.Scenario{
		{ID: "s1", Name: "A"},
		{ID: "s2", Name: "B"},
	})
	s.Activate("s1")

	active := s.ActiveScenarios()
	if len(active) != 1 || active[0].Name != "A" {
		t.Fatalf("unexpected active scenarios: %v", active)
	}
}

func TestStore_PatchFor_WhenActive(t *testing.T) {
	delay := config.Duration{}
	delay.Duration = 2 * time.Second
	sc := config.Scenario{
		ID: "s1",
		Patches: []config.MockPatch{
			{MockID: "m1", Status: 503, Body: "down", Delay: &delay},
			{MockID: "m2", Status: 429},
		},
	}
	s := scenarios.New([]config.Scenario{sc})

	// Not active yet — no patch
	if p := s.PatchFor("m1"); p != nil {
		t.Fatal("expected nil patch when scenario not active")
	}

	s.Activate("s1")

	p := s.PatchFor("m1")
	if p == nil {
		t.Fatal("expected patch for m1 after activation")
	}
	if p.Status != 503 {
		t.Errorf("want status 503, got %d", p.Status)
	}
	if p.Body != "down" {
		t.Errorf("want body 'down', got %q", p.Body)
	}
	if p.Delay == nil || p.Delay.Duration != 2*time.Second {
		t.Errorf("unexpected delay: %v", p.Delay)
	}

	p2 := s.PatchFor("m2")
	if p2 == nil || p2.Status != 429 {
		t.Fatalf("unexpected patch for m2: %v", p2)
	}
}

func TestStore_PatchFor_UnknownMock(t *testing.T) {
	s := scenarios.New([]config.Scenario{{ID: "s1", Patches: []config.MockPatch{{MockID: "m1"}}}})
	s.Activate("s1")
	if p := s.PatchFor("nonexistent"); p != nil {
		t.Fatal("expected nil for unknown mock")
	}
}

func TestStore_DirectFaults_SetClear(t *testing.T) {
	s := scenarios.New(nil)

	if got := s.DirectFaults(); got.HTTP != nil {
		t.Fatal("expected nil faults initially")
	}

	s.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Status: 503}})

	got := s.DirectFaults()
	if got.HTTP == nil || got.HTTP.Status != 503 {
		t.Fatalf("unexpected faults: %+v", got)
	}

	got.HTTP = nil
	if s.DirectFaults().HTTP == nil {
		t.Fatal("fault copy should be preserved")
	}

	s.ClearDirectFaults()
	if got := s.DirectFaults(); got.HTTP != nil {
		t.Fatal("expected nil after ClearDirectFaults")
	}
}

func TestStore_EffectiveHTTPFault_PrioritizesScenario(t *testing.T) {
	s := scenarios.New([]config.Scenario{{
		ID:     "b",
		Faults: &config.ProtocolFaults{HTTP: &config.HTTPFault{Status: 502}},
	}, {
		ID:     "a",
		Faults: &config.ProtocolFaults{HTTP: &config.HTTPFault{Status: 501}},
	}})
	s.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Status: 503}})
	s.Activate("a")
	s.Activate("b")

	got := s.EffectiveHTTPFault()
	if got == nil || got.Status != 502 {
		t.Fatalf("unexpected effective fault: %+v", got)
	}
}

func TestStore_FaultRoll(t *testing.T) {
	s := scenarios.New(nil)

	if !s.ShouldFault(0) {
		t.Error("rate=0 should always apply")
	}
	if !s.ShouldFault(1.0) {
		t.Error("rate=1.0 should always apply")
	}
	if !s.ShouldFault(-1) {
		t.Error("rate<0 should always apply")
	}

	// 0.5 rate: probabilistic — run many trials and verify it's not always the same.
	hits := 0
	for i := 0; i < 200; i++ {
		if s.ShouldFault(0.5) {
			hits++
		}
	}
	// With rate 0.5 over 200 trials, expect between 60 and 140 hits (very wide margin).
	if hits < 60 || hits > 140 {
		t.Errorf("rate=0.5 over 200 trials: expected ~100 hits, got %d", hits)
	}
}

// ---------------------------------------------------------------------------
// New — auto-ID generation when scenario has no ID
// ---------------------------------------------------------------------------

func TestStore_New_AutoIDForEmptyID(t *testing.T) {
	s := scenarios.New([]config.Scenario{
		{Name: "unnamed"},          // empty ID → should get auto-generated
		{ID: "explicit", Name: "explicit"},
	})
	all := s.All()
	if len(all) != 2 {
		t.Fatalf("want 2 scenarios, got %d", len(all))
	}
	for _, sc := range all {
		if sc.ID == "" {
			t.Error("all scenarios must have a non-empty ID")
		}
	}
	// The explicit ID must be preserved.
	found := false
	for _, sc := range all {
		if sc.ID == "explicit" {
			found = true
		}
	}
	if !found {
		t.Error("explicit scenario ID must be preserved")
	}
}

// ---------------------------------------------------------------------------
// PatchFor — active scenario that was deleted from the store
// ---------------------------------------------------------------------------

func TestStore_PatchFor_DeletedWhileActive(t *testing.T) {
	s := scenarios.New([]config.Scenario{{
		ID:      "ghost",
		Patches: []config.MockPatch{{MockID: "m1", Status: 500}},
	}})
	s.Activate("ghost")

	// Delete the scenario while it's still in the active set.
	s.Delete("ghost")

	// PatchFor should safely skip the stale active ID and return nil.
	patch := s.PatchFor("m1")
	if patch != nil {
		t.Errorf("expected nil patch for deleted scenario, got %+v", patch)
	}
}

// ---------------------------------------------------------------------------
// effectiveFault — nil store receiver
// ---------------------------------------------------------------------------

func TestEffectiveFault_NilStore(t *testing.T) {
	var s *scenarios.Store
	if got := s.EffectiveHTTPFault(); got != nil {
		t.Errorf("nil store: want nil fault, got %+v", got)
	}
	if got := s.EffectiveDNSFault(); got != nil {
		t.Errorf("nil store: want nil DNS fault, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// effectiveFault — scenario with Faults but not the queried protocol
// ---------------------------------------------------------------------------

func TestEffectiveFault_ScenarioHasFaultsButNotThisProtocol(t *testing.T) {
	// Activate a scenario that has DNS fault but not HTTP fault.
	// EffectiveHTTPFault should fall back to direct fault.
	s := scenarios.New([]config.Scenario{{
		ID:     "dns-only",
		Faults: &config.ProtocolFaults{DNS: &config.DNSFault{Rcode: "NXDOMAIN"}},
	}})
	s.SetDirectFaults(config.ProtocolFaults{HTTP: &config.HTTPFault{Status: 503}})
	s.Activate("dns-only")

	got := s.EffectiveHTTPFault()
	if got == nil || got.Status != 503 {
		t.Errorf("want direct HTTP fault (503), got %+v", got)
	}
	if s.EffectiveDNSFault() == nil {
		t.Error("expected DNS fault from active scenario")
	}
}
