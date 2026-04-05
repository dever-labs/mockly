package engine_test

import (
	"strconv"
	"strings"
	"testing"
	"text/template"

	"github.com/dever-labs/mockly/internal/engine"
)

// execute runs a template string through BuildFuncMap and returns the result.
func execute(t *testing.T, tmpl string) string {
	t.Helper()
	fm := engine.BuildFuncMap()
	tpl, err := template.New("t").Funcs(fm).Parse(tmpl)
	if err != nil {
		t.Fatalf("parse %q: %v", tmpl, err)
	}
	var sb strings.Builder
	if err := tpl.Execute(&sb, nil); err != nil {
		t.Fatalf("execute %q: %v", tmpl, err)
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// uuid
// ---------------------------------------------------------------------------

func TestFuncs_UUID(t *testing.T) {
	out := execute(t, `{{uuid}}`)
	if len(out) != 36 {
		t.Errorf("uuid: expected 36 chars, got %d (%q)", len(out), out)
	}
	if out[8] != '-' || out[13] != '-' || out[18] != '-' || out[23] != '-' {
		t.Errorf("uuid: wrong format: %q", out)
	}
	out2 := execute(t, `{{uuid}}`)
	if out == out2 {
		t.Error("uuid: two calls should produce different values")
	}
}

// ---------------------------------------------------------------------------
// rand_int
// ---------------------------------------------------------------------------

func TestFuncs_RandInt(t *testing.T) {
	for range 50 {
		out := execute(t, `{{rand_int 1 10}}`)
		n, err := strconv.Atoi(out)
		if err != nil {
			t.Fatalf("rand_int: not an int: %q", out)
		}
		if n < 1 || n > 10 {
			t.Errorf("rand_int 1 10: %d out of range", n)
		}
	}
}

func TestFuncs_RandInt_SameMinMax(t *testing.T) {
	out := execute(t, `{{rand_int 7 7}}`)
	if out != "7" {
		t.Errorf("rand_int 7 7: expected 7, got %q", out)
	}
}

// ---------------------------------------------------------------------------
// rand_float
// ---------------------------------------------------------------------------

func TestFuncs_RandFloat(t *testing.T) {
	out := execute(t, `{{rand_float 0.0 1.0 2}}`)
	f, err := strconv.ParseFloat(out, 64)
	if err != nil {
		t.Fatalf("rand_float: not a float: %q", out)
	}
	if f < 0.0 || f > 1.0 {
		t.Errorf("rand_float 0 1: %f out of range", f)
	}
	// Check decimal places
	parts := strings.Split(out, ".")
	if len(parts) != 2 || len(parts[1]) > 2 {
		t.Errorf("rand_float 0 1 2: expected 2 decimal places, got %q", out)
	}
}

// ---------------------------------------------------------------------------
// rand_string
// ---------------------------------------------------------------------------

func TestFuncs_RandString_Default(t *testing.T) {
	out := execute(t, `{{rand_string 12}}`)
	if len(out) != 12 {
		t.Errorf("rand_string 12: expected 12 chars, got %d", len(out))
	}
}

func TestFuncs_RandString_Hex(t *testing.T) {
	out := execute(t, `{{rand_string 8 "hex"}}`)
	if len(out) != 8 {
		t.Errorf("expected 8 chars, got %d", len(out))
	}
	for _, c := range out {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("rand_string hex: unexpected char %q in %q", c, out)
		}
	}
}

func TestFuncs_RandString_Numeric(t *testing.T) {
	out := execute(t, `{{rand_string 6 "numeric"}}`)
	for _, c := range out {
		if c < '0' || c > '9' {
			t.Errorf("rand_string numeric: unexpected char %q in %q", c, out)
		}
	}
}

// ---------------------------------------------------------------------------
// rand_bool
// ---------------------------------------------------------------------------

func TestFuncs_RandBool(t *testing.T) {
	seen := map[string]bool{}
	for range 50 {
		out := execute(t, `{{rand_bool}}`)
		if out != "true" && out != "false" {
			t.Fatalf("rand_bool: unexpected value %q", out)
		}
		seen[out] = true
	}
	if !seen["true"] || !seen["false"] {
		t.Error("rand_bool: expected to see both true and false over 50 trials")
	}
}

// ---------------------------------------------------------------------------
// pick
// ---------------------------------------------------------------------------

func TestFuncs_Pick(t *testing.T) {
	seen := map[string]bool{}
	for range 60 {
		out := execute(t, `{{pick "red" "green" "blue"}}`)
		seen[out] = true
	}
	for _, want := range []string{"red", "green", "blue"} {
		if !seen[want] {
			t.Errorf("pick: never saw %q in 60 trials", want)
		}
	}
}

// ---------------------------------------------------------------------------
// fake
// ---------------------------------------------------------------------------

func TestFuncs_Fake_Name(t *testing.T) {
	out := execute(t, `{{fake "name"}}`)
	parts := strings.Fields(out)
	if len(parts) != 2 {
		t.Errorf("fake name: expected 2 words, got %q", out)
	}
}

func TestFuncs_Fake_Email(t *testing.T) {
	out := execute(t, `{{fake "email"}}`)
	if !strings.Contains(out, "@") || !strings.Contains(out, ".") {
		t.Errorf("fake email: looks wrong: %q", out)
	}
}

func TestFuncs_Fake_IP(t *testing.T) {
	out := execute(t, `{{fake "ip"}}`)
	if !strings.HasPrefix(out, "192.168.") {
		t.Errorf("fake ip: expected 192.168.x.x, got %q", out)
	}
}

func TestFuncs_Fake_IPv6(t *testing.T) {
	out := execute(t, `{{fake "ipv6"}}`)
	if !strings.HasPrefix(out, "2001:db8::") {
		t.Errorf("fake ipv6: expected 2001:db8::, got %q", out)
	}
}

func TestFuncs_Fake_URL(t *testing.T) {
	out := execute(t, `{{fake "url"}}`)
	if !strings.HasPrefix(out, "https://") {
		t.Errorf("fake url: expected https://, got %q", out)
	}
}

func TestFuncs_Fake_Phone(t *testing.T) {
	out := execute(t, `{{fake "phone"}}`)
	if !strings.HasPrefix(out, "+1-555-") {
		t.Errorf("fake phone: expected +1-555-xxxx, got %q", out)
	}
}

func TestFuncs_Fake_Unknown(t *testing.T) {
	fm := engine.BuildFuncMap()
	tpl, _ := template.New("t").Funcs(fm).Parse(`{{fake "nonexistent"}}`)
	var sb strings.Builder
	err := tpl.Execute(&sb, nil)
	if err == nil {
		t.Error("fake unknown: expected error, got nil")
	}
}

func TestFuncs_Fake_AllKinds(t *testing.T) {
	kinds := []string{
		"name", "first_name", "last_name", "email", "username", "phone",
		"company", "city", "country", "street", "zip", "ip", "ipv6",
		"url", "useragent", "word", "sentence",
	}
	for _, kind := range kinds {
		out := execute(t, `{{fake "`+kind+`"}}`)
		if out == "" {
			t.Errorf("fake %q returned empty string", kind)
		}
	}
}

// ---------------------------------------------------------------------------
// seq
// ---------------------------------------------------------------------------

func TestFuncs_Seq(t *testing.T) {
	engine.ResetSequences()
	for i := range 5 {
		out := execute(t, `{{seq "test-counter"}}`)
		want := strconv.Itoa(i + 1)
		if out != want {
			t.Errorf("seq call %d: want %q, got %q", i+1, want, out)
		}
	}
}

func TestFuncs_Seq_Independent(t *testing.T) {
	engine.ResetSequences()
	execute(t, `{{seq "a"}}`) // a=1
	execute(t, `{{seq "a"}}`) // a=2
	execute(t, `{{seq "b"}}`) // b=1
	outA := execute(t, `{{seq "a"}}`) // a=3
	outB := execute(t, `{{seq "b"}}`) // b=2
	if outA != "3" {
		t.Errorf("seq a: want 3, got %q", outA)
	}
	if outB != "2" {
		t.Errorf("seq b: want 2, got %q", outB)
	}
}

func TestFuncs_ResetSequences(t *testing.T) {
	engine.ResetSequences()
	execute(t, `{{seq "x"}}`) // x=1
	execute(t, `{{seq "x"}}`) // x=2
	engine.ResetSequences()
	out := execute(t, `{{seq "x"}}`) // x=1 again
	if out != "1" {
		t.Errorf("after reset, seq x: want 1, got %q", out)
	}
}

// ---------------------------------------------------------------------------
// lorem
// ---------------------------------------------------------------------------

func TestFuncs_Lorem(t *testing.T) {
	out := execute(t, `{{lorem 5}}`)
	words := strings.Fields(out)
	if len(words) != 5 {
		t.Errorf("lorem 5: expected 5 words, got %d (%q)", len(words), out)
	}
}

func TestFuncs_Lorem_Zero(t *testing.T) {
	out := execute(t, `{{lorem 0}}`)
	if out != "" {
		t.Errorf("lorem 0: expected empty, got %q", out)
	}
}

// ---------------------------------------------------------------------------
// date
// ---------------------------------------------------------------------------

func TestFuncs_Date(t *testing.T) {
	out := execute(t, `{{date "2006-01-02"}}`)
	if len(out) != 10 || out[4] != '-' || out[7] != '-' {
		t.Errorf("date: expected YYYY-MM-DD, got %q", out)
	}
}

func TestFuncs_DateAdd(t *testing.T) {
	out := execute(t, `{{date_add "2006-01-02" "0s"}}`)
	if len(out) != 10 {
		t.Errorf("date_add: expected YYYY-MM-DD, got %q", out)
	}
}

// ---------------------------------------------------------------------------
// now / upper / lower (regression)
// ---------------------------------------------------------------------------

func TestFuncs_Now(t *testing.T) {
	out := execute(t, `{{now}}`)
	if len(out) < 20 {
		t.Errorf("now: too short: %q", out)
	}
}

func TestFuncs_UpperLower(t *testing.T) {
	if out := execute(t, `{{upper "hello"}}`); out != "HELLO" {
		t.Errorf("upper: got %q", out)
	}
	if out := execute(t, `{{lower "WORLD"}}`); out != "world" {
		t.Errorf("lower: got %q", out)
	}
}
