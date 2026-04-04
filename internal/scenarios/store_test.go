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

func TestStore_Fault_SetGetClear(t *testing.T) {
	s := scenarios.New(nil)

	if s.GetFault() != nil {
		t.Fatal("expected nil fault initially")
	}

	f := &config.GlobalFault{Enabled: true, StatusOverride: 503}
	s.SetFault(f)

	got := s.GetFault()
	if got == nil || got.StatusOverride != 503 {
		t.Fatalf("unexpected fault: %v", got)
	}

	// Returned value should be a copy, not a pointer to internal state.
	got.StatusOverride = 999
	if s.GetFault().StatusOverride != 503 {
		t.Fatal("GetFault should return a copy")
	}

	s.ClearFault()
	if s.GetFault() != nil {
		t.Fatal("expected nil after ClearFault")
	}
}

func TestStore_RollFault(t *testing.T) {
	s := scenarios.New(nil)

	if !s.RollFault(0) {
		t.Error("rate=0 should always apply fault")
	}
	if !s.RollFault(1.0) {
		t.Error("rate=1.0 should always apply fault")
	}
	if !s.RollFault(-1) {
		t.Error("rate<0 should always apply fault")
	}

	// 0.5 rate: probabilistic — run many trials and verify it's not always the same.
	hits := 0
	for i := 0; i < 200; i++ {
		if s.RollFault(0.5) {
			hits++
		}
	}
	// With rate 0.5 over 200 trials, expect between 60 and 140 hits (very wide margin).
	if hits < 60 || hits > 140 {
		t.Errorf("rate=0.5 over 200 trials: expected ~100 hits, got %d", hits)
	}
}
