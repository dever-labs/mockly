// Internal package test so we can access unexported helpers.
package redisserver

import (
	"testing"
)

func TestMatchRedisKey_Any(t *testing.T) {
	if !matchRedisKey("*", "anything") {
		t.Error("* should match any key")
	}
	if !matchRedisKey("*", "") {
		t.Error("* should match empty key")
	}
}

func TestMatchRedisKey_Exact(t *testing.T) {
	if !matchRedisKey("user:123", "user:123") {
		t.Error("exact key should match")
	}
	if matchRedisKey("user:123", "user:456") {
		t.Error("exact key should not match different value")
	}
}

func TestMatchRedisKey_Glob(t *testing.T) {
	if !matchRedisKey("user:*", "user:123") {
		t.Error("glob user:* should match user:123")
	}
	if !matchRedisKey("user:*", "user:anything") {
		t.Error("glob user:* should match user:anything")
	}
	if matchRedisKey("user:*", "session:123") {
		t.Error("glob user:* should not match session:123")
	}
}

func TestMatchRedisKey_GlobSuffix(t *testing.T) {
	if !matchRedisKey("*:active", "user:active") {
		t.Error("suffix glob should match")
	}
	if matchRedisKey("*:active", "user:inactive") {
		t.Error("suffix glob should not match wrong suffix")
	}
}

func TestMatchRedisKey_Regex(t *testing.T) {
	if !matchRedisKey(`re:^user:\d+$`, "user:123") {
		t.Error("regex should match numeric user key")
	}
	if matchRedisKey(`re:^user:\d+$`, "user:abc") {
		t.Error("regex should not match non-numeric user key")
	}
	if matchRedisKey("re:[bad", "anything") {
		t.Error("invalid regex should not match")
	}
}

func TestToStringSlice(t *testing.T) {
	cases := []struct {
		in   interface{}
		want int
	}{
		{[]interface{}{"a", "b", "c"}, 3},
		{[]string{"x", "y"}, 2},
		{nil, 0},
		{"not a slice", 0},
	}
	for _, c := range cases {
		got := toStringSlice(c.in)
		if len(got) != c.want {
			t.Errorf("toStringSlice(%v) len = %d, want %d", c.in, len(got), c.want)
		}
	}
}
