package engine

import (
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

// rng is a package-level PRNG used exclusively for generating mock/fake data.
// Cryptographic randomness is not required here — the only goal is variety in
// generated test data. #nosec G404
var rng = rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404 -- mock data generation does not require crypto randomness
var rngMu sync.Mutex

// BuildFuncMap returns the complete set of template functions available in
// mock response bodies. Functions are safe for concurrent use.
func BuildFuncMap() template.FuncMap {
	return template.FuncMap{
		// ── time ────────────────────────────────────────────────────────────
		"now":      func() string { return time.Now().UTC().Format(time.RFC3339) },
		"date":     fnDate,     // date "2006-01-02"
		"date_add": fnDateAdd,  // date_add "2006-01-02" "24h"

		// ── string transforms ────────────────────────────────────────────────
		"upper": strings.ToUpper,
		"lower": strings.ToLower,

		// ── random primitives ────────────────────────────────────────────────
		"uuid":        fnUUID,        // uuid
		"rand_int":    fnRandInt,     // rand_int MIN MAX
		"rand_float":  fnRandFloat,   // rand_float MIN MAX DECIMALS
		"rand_string": fnRandString,  // rand_string LENGTH [charset]
		"rand_bool":   fnRandBool,    // rand_bool

		// ── selection ────────────────────────────────────────────────────────
		"pick": fnPick, // pick "a" "b" "c" ...

		// ── fake structured data ─────────────────────────────────────────────
		"fake": fnFake, // fake "name|email|phone|company|city|country|zip|ip|ipv6|url|username|useragent|word|sentence"

		// ── sequences ────────────────────────────────────────────────────────
		"seq": fnSeq, // seq "counter_name" → 1, 2, 3, ...

		// ── text ─────────────────────────────────────────────────────────────
		"lorem": fnLorem, // lorem N
	}
}

// ResetSequences resets all named sequence counters to zero.
// Called by the management API /api/reset endpoint.
func ResetSequences() {
	seqMu.Lock()
	defer seqMu.Unlock()
	seqCounters = make(map[string]int64)
}

// ---------------------------------------------------------------------------
// Time
// ---------------------------------------------------------------------------

func fnDate(format string) string {
	return time.Now().UTC().Format(format)
}

func fnDateAdd(format, duration string) (string, error) {
	d, err := time.ParseDuration(duration)
	if err != nil {
		return "", fmt.Errorf("date_add: invalid duration %q: %w", duration, err)
	}
	return time.Now().UTC().Add(d).Format(format), nil
}

// ---------------------------------------------------------------------------
// Random primitives
// ---------------------------------------------------------------------------

// fnUUID generates a random UUID v4 string using crypto/rand.
func fnUUID() string {
	var b [16]byte
	_, _ = crand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

const (
	charsetAlphaLower    = "abcdefghijklmnopqrstuvwxyz"
	charsetAlphaUpper    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	charsetAlpha         = charsetAlphaLower + charsetAlphaUpper
	charsetNumeric       = "0123456789"
	charsetAlphanumeric  = charsetAlpha + charsetNumeric
	charsetAlphanumLower = charsetAlphaLower + charsetNumeric
	charsetHex           = "0123456789abcdef"
)

// rngIntn returns a non-negative random int in [0,n). Safe for concurrent use.
func rngIntn(n int) int {
	rngMu.Lock()
	v := rng.Intn(n)
	rngMu.Unlock()
	return v
}

// rngFloat64 returns a random float64 in [0.0,1.0). Safe for concurrent use.
func rngFloat64() float64 {
	rngMu.Lock()
	v := rng.Float64()
	rngMu.Unlock()
	return v
}

func fnRandInt(min, max int) (string, error) {
	if min > max {
		return "", fmt.Errorf("rand_int: min (%d) > max (%d)", min, max)
	}
	return strconv.Itoa(min + rngIntn(max-min+1)), nil
}

func fnRandFloat(min, max float64, decimals int) string {
	v := min + rngFloat64()*(max-min)
	return strconv.FormatFloat(v, 'f', decimals, 64)
}

// fnRandString generates a random string of the given length.
// An optional second argument selects the character set:
//
//	"alpha"        — a-zA-Z
//	"lower"        — a-z
//	"upper"        — A-Z
//	"numeric"      — 0-9
//	"alphanumeric" — a-zA-Z0-9 (default)
//	"hex"          — 0-9a-f
//	any other string is treated as a custom character set
func fnRandString(length int, args ...string) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("rand_string: length must be > 0, got %d", length)
	}
	charset := charsetAlphanumeric
	if len(args) > 0 {
		switch args[0] {
		case "alpha":
			charset = charsetAlpha
		case "lower":
			charset = charsetAlphaLower
		case "upper":
			charset = charsetAlphaUpper
		case "numeric":
			charset = charsetNumeric
		case "alphanumeric":
			charset = charsetAlphanumeric
		case "hex":
			charset = charsetHex
		default:
			if len(args[0]) == 0 {
				return "", fmt.Errorf("rand_string: custom charset is empty")
			}
			charset = args[0]
		}
	}
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rngIntn(len(charset))]
	}
	return string(b), nil
}

func fnRandBool() string {
	if rngIntn(2) == 0 {
		return "false"
	}
	return "true"
}

func fnPick(options ...string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("pick: requires at least one option")
	}
	return options[rngIntn(len(options))], nil
}

// ---------------------------------------------------------------------------
// Fake structured data
// ---------------------------------------------------------------------------

var (
	fakeFirstNames = []string{
		"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank", "Grace", "Henry",
		"Iris", "Jack", "Kate", "Liam", "Mia", "Noah", "Olivia", "Paul",
		"Quinn", "Ruby", "Sam", "Tara",
	}
	fakeLastNames = []string{
		"Smith", "Jones", "Williams", "Brown", "Taylor", "Davies", "Evans",
		"Wilson", "Thomas", "Roberts", "Johnson", "White", "Martin", "Anderson",
		"Thompson", "Garcia", "Martinez", "Robinson", "Clark", "Lewis",
	}
	fakeCompanyPrefix = []string{
		"Acme", "Apex", "Atlas", "Blue", "Bright", "Core", "Delta", "Edge",
		"Fast", "Global", "Horizon", "Ionic", "Matrix", "Nova", "Zenith",
	}
	fakeCompanySuffix = []string{
		"Inc", "LLC", "Corp", "Group", "Labs", "Solutions", "Systems", "Technologies",
	}
	fakeCities = []string{
		"London", "New York", "Berlin", "Tokyo", "Paris", "Sydney", "Toronto",
		"Singapore", "Dubai", "Amsterdam", "Chicago", "Madrid", "Mumbai", "Seoul", "Austin",
	}
	fakeCountries = []string{
		"United States", "Germany", "Japan", "United Kingdom", "France", "Australia",
		"Canada", "Singapore", "Netherlands", "Brazil", "India", "Spain",
		"South Korea", "Mexico", "Sweden",
	}
	fakeEmailDomains = []string{
		"example.com", "test.io", "mockly.dev", "demo.net", "fake.org",
		"sandbox.io", "acme.com", "corp.net", "devlab.io", "staging.co",
	}
	fakeUserAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_4) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15",
		"Mozilla/5.0 (X11; Linux x86_64; rv:125.0) Gecko/20100101 Firefox/125.0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148 Safari/604.1",
		"curl/8.6.0",
	}
	fakeWords = []string{
		"lorem", "ipsum", "dolor", "sit", "amet", "consectetur", "adipiscing",
		"elit", "sed", "do", "eiusmod", "tempor", "incididunt", "ut", "labore",
		"et", "dolore", "magna", "aliqua", "enim", "minim", "veniam", "quis",
		"nostrud", "exercitation", "ullamco", "laboris",
	}
	fakeStreets = []string{
		"Main St", "Oak Ave", "Maple Rd", "Elm St", "Cedar Blvd",
		"Park Ave", "Broadway", "Market St", "High St", "Church Lane",
	}
	fakeTLDs = []string{"com", "io", "dev", "net", "org", "co"}
)

func randPick(list []string) string {
	return list[rngIntn(len(list))]
}

func fnFake(kind string) (string, error) {
	switch kind {
	case "name":
		return randPick(fakeFirstNames) + " " + randPick(fakeLastNames), nil
	case "first_name":
		return randPick(fakeFirstNames), nil
	case "last_name":
		return randPick(fakeLastNames), nil
	case "email":
		first := strings.ToLower(randPick(fakeFirstNames))
		last := strings.ToLower(randPick(fakeLastNames))
		return fmt.Sprintf("%s.%s@%s", first, last, randPick(fakeEmailDomains)), nil
	case "username":
		first := strings.ToLower(randPick(fakeFirstNames))
		n := 10 + rngIntn(90)
		return fmt.Sprintf("%s%d", first, n), nil
	case "phone":
		return fmt.Sprintf("+1-555-%04d", rngIntn(10000)), nil
	case "company":
		return randPick(fakeCompanyPrefix) + " " + randPick(fakeCompanySuffix), nil
	case "city":
		return randPick(fakeCities), nil
	case "country":
		return randPick(fakeCountries), nil
	case "street":
		n := 1 + rngIntn(999)
		return fmt.Sprintf("%d %s", n, randPick(fakeStreets)), nil
	case "zip":
		return fmt.Sprintf("%05d", rngIntn(100000)), nil
	case "ip":
		return fmt.Sprintf("192.168.%d.%d", rngIntn(256), 1+rngIntn(254)), nil
	case "ipv6":
		return fmt.Sprintf("2001:db8::%04x:%04x", rngIntn(0x10000), rngIntn(0x10000)), nil
	case "url":
		path := randPick(fakeWords)
		tld := randPick(fakeTLDs)
		company := strings.ToLower(randPick(fakeCompanyPrefix))
		return fmt.Sprintf("https://%s.%s/api/%s", company, tld, path), nil
	case "useragent":
		return randPick(fakeUserAgents), nil
	case "word":
		return randPick(fakeWords), nil
	case "sentence":
		n := 5 + rngIntn(6)
		return fnLorem(n), nil
	default:
		return "", fmt.Errorf("fake: unknown kind %q — valid kinds: name, first_name, last_name, email, username, phone, company, city, country, street, zip, ip, ipv6, url, useragent, word, sentence", kind)
	}
}

// ---------------------------------------------------------------------------
// Sequences
// ---------------------------------------------------------------------------

var (
	seqMu       sync.Mutex
	seqCounters = make(map[string]int64)
)

// fnSeq returns the next value of a named monotonic counter (starts at 1).
func fnSeq(name string) int64 {
	seqMu.Lock()
	defer seqMu.Unlock()
	seqCounters[name]++
	return seqCounters[name]
}

// ---------------------------------------------------------------------------
// Text
// ---------------------------------------------------------------------------

// fnLorem returns n space-joined lorem ipsum words.
func fnLorem(n int) string {
	if n <= 0 {
		return ""
	}
	words := make([]string, n)
	for i := range words {
		words[i] = fakeWords[rngIntn(len(fakeWords))]
	}
	return strings.Join(words, " ")
}
