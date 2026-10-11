package snmpserver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// byteString is a parsed OID: one int per dotted component. Comparing OIDs
// component-wise numerically (rather than as raw strings) is required for
// correct ordering — e.g. the component 10 must sort after 2, which a plain
// string comparison would get backwards.
type byteString []int

// oidToByteString parses a dotted OID string (e.g. "1.3.6.1.2.1.1.1.0",
// optionally with a leading dot) into its numeric components.
func oidToByteString(oid string) (byteString, error) {
	parts := strings.Split(oid, ".")
	out := make([]int, 0, len(parts))
	for i, p := range parts {
		if p == "" {
			if i == 0 {
				continue // tolerate a leading "." (e.g. ".1.3.6...")
			}
			return nil, fmt.Errorf("invalid oid %q: empty component", oid)
		}
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid oid %q: %w", oid, err)
		}
		out = append(out, int(n))
	}
	return out, nil
}

// compareByteString returns -1, 0, or 1 as a < b, a == b, or a > b.
func compareByteString(a, b byteString) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		switch {
		case a[i] < b[i]:
			return -1
		case a[i] > b[i]:
			return 1
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}

// sortOIDs sorts items into numeric OID order, which GetNext/GetBulk walks
// depend on to find the "next" OID after a given query.
func sortOIDs(items []*pduItem) {
	sort.Slice(items, func(i, j int) bool {
		bi, _ := oidToByteString(items[i].oid)
		bj, _ := oidToByteString(items[j].oid)
		return compareByteString(bi, bj) < 0
	})
}

// findOID looks up oid in items (which must already be sorted by sortOIDs)
// and returns the exact match and its index, or (nil, insertion index) if no
// exact match exists — the insertion index is the index of the first item
// numerically greater than oid, which GetNext/GetBulk use to find the next
// walkable OID after a non-existent query OID.
func findOID(items []*pduItem, oid string) (*pduItem, int) {
	query, err := oidToByteString(oid)
	if err != nil {
		return nil, len(items)
	}
	idx := sort.Search(len(items), func(i int) bool {
		b, _ := oidToByteString(items[i].oid)
		return compareByteString(b, query) >= 0
	})
	if idx < len(items) {
		b, _ := oidToByteString(items[idx].oid)
		if compareByteString(b, query) == 0 {
			return items[idx], idx
		}
	}
	return nil, idx
}
