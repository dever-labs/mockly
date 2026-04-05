package state_test

import (
	"sync"
	"testing"

	"github.com/dever-labs/mockly/internal/state"
)

func TestNew(t *testing.T) {
	s := state.New()
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if all := s.All(); len(all) != 0 {
		t.Fatalf("new store should be empty, got %d entries", len(all))
	}
}

func TestSet_Get_Present(t *testing.T) {
	s := state.New()
	s.Set("key", "value")

	v, ok := s.Get("key")
	if !ok {
		t.Fatal("expected to find key")
	}
	if v != "value" {
		t.Errorf("want %q, got %q", "value", v)
	}
}

func TestGet_Absent(t *testing.T) {
	s := state.New()
	v, ok := s.Get("missing")
	if ok {
		t.Fatal("expected ok=false for absent key")
	}
	if v != "" {
		t.Errorf("expected empty string for absent key, got %q", v)
	}
}

func TestDelete_Present(t *testing.T) {
	s := state.New()
	s.Set("k", "v")
	s.Delete("k")

	_, ok := s.Get("k")
	if ok {
		t.Fatal("key should be absent after Delete")
	}
}

func TestDelete_Absent(t *testing.T) {
	s := state.New()
	// Deleting a key that doesn't exist should be a no-op (no panic).
	s.Delete("nonexistent")
	if all := s.All(); len(all) != 0 {
		t.Fatalf("store should be empty, got %d entries", len(all))
	}
}

func TestAll_Empty(t *testing.T) {
	s := state.New()
	all := s.All()
	if all == nil {
		t.Fatal("All() should return non-nil map")
	}
	if len(all) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(all))
	}
}

func TestAll_Populated(t *testing.T) {
	s := state.New()
	s.Set("a", "1")
	s.Set("b", "2")

	all := s.All()
	if len(all) != 2 {
		t.Fatalf("want 2 entries, got %d", len(all))
	}
	if all["a"] != "1" || all["b"] != "2" {
		t.Errorf("unexpected entries: %v", all)
	}

	// Verify returned map is a snapshot (not live).
	all["a"] = "mutated"
	if v, _ := s.Get("a"); v != "1" {
		t.Error("All() should return a copy, not a reference to internal state")
	}
}

func TestReset(t *testing.T) {
	s := state.New()
	s.Set("x", "1")
	s.Set("y", "2")
	s.Reset()

	if all := s.All(); len(all) != 0 {
		t.Fatalf("expected empty store after Reset, got %d entries", len(all))
	}

	// Verify we can still Set after Reset.
	s.Set("z", "3")
	if v, ok := s.Get("z"); !ok || v != "3" {
		t.Errorf("store should be usable after Reset, got v=%q ok=%v", v, ok)
	}
}

func TestConcurrentWrites(t *testing.T) {
	s := state.New()
	const goroutines = 50
	const writes = 20

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		g := g
		go func() {
			defer wg.Done()
			for i := 0; i < writes; i++ {
				key := string(rune('a' + g%26))
				s.Set(key, "v")
				s.Get(key)
				s.Delete(key)
				s.All()
			}
		}()
	}
	wg.Wait()
	// No race, no panic — test passes if we reach here.
}
