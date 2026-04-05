package presets_test

import (
	"strings"
	"testing"

	"github.com/dever-labs/mockly/internal/presets"
)

func TestAll_AtLeastNineItems(t *testing.T) {
	if len(presets.All) < 9 {
		t.Fatalf("expected at least 9 presets, got %d", len(presets.All))
	}
}

func TestAll_NonEmptyFields(t *testing.T) {
	for _, p := range presets.All {
		if p.Name == "" {
			t.Errorf("preset %v has empty Name", p)
		}
		if p.Description == "" {
			t.Errorf("preset %q has empty Description", p.Name)
		}
		if p.Filename == "" {
			t.Errorf("preset %q has empty Filename", p.Name)
		}
	}
}

func TestFind_ByName(t *testing.T) {
	p, err := presets.Find("keycloak")
	if err != nil {
		t.Fatalf("Find(keycloak) unexpected error: %v", err)
	}
	if p.Name != "keycloak" {
		t.Errorf("want name keycloak, got %q", p.Name)
	}
}

func TestFind_CaseInsensitive(t *testing.T) {
	p, err := presets.Find("STRIPE")
	if err != nil {
		t.Fatalf("Find(STRIPE) unexpected error: %v", err)
	}
	if p.Name != "stripe" {
		t.Errorf("want name stripe, got %q", p.Name)
	}

	p2, err2 := presets.Find("  OpenAI  ")
	if err2 != nil {
		t.Fatalf("Find(  OpenAI  ) unexpected error: %v", err2)
	}
	if p2.Name != "openai" {
		t.Errorf("want name openai, got %q", p2.Name)
	}
}

func TestFind_Unknown(t *testing.T) {
	_, err := presets.Find("doesnotexist")
	if err == nil {
		t.Fatal("expected error for unknown preset, got nil")
	}
	if !strings.Contains(err.Error(), "doesnotexist") {
		t.Errorf("error message should contain the requested name, got: %v", err)
	}
}

func TestRead_ValidPreset(t *testing.T) {
	data, err := presets.Read("github")
	if err != nil {
		t.Fatalf("Read(github) unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Read(github) returned empty bytes")
	}
	// Should be valid YAML-ish content — at minimum non-empty.
	if !strings.Contains(string(data), "github") && !strings.Contains(string(data), "http") {
		t.Errorf("github preset data looks unexpected: %q...", string(data[:min(100, len(data))]))
	}
}

func TestRead_AllPresetsHaveContent(t *testing.T) {
	for _, p := range presets.All {
		data, err := presets.Read(p.Name)
		if err != nil {
			t.Errorf("Read(%q) unexpected error: %v", p.Name, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("Read(%q) returned empty bytes", p.Name)
		}
	}
}

func TestRead_Invalid(t *testing.T) {
	_, err := presets.Read("nonexistent-preset")
	if err == nil {
		t.Fatal("expected error for invalid preset name, got nil")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
