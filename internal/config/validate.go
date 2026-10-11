package config

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"
)

// Validate performs structural checks on a loaded Config beyond plain YAML
// parsing: duplicate mock IDs within a single protocol's mock list, and
// invalid regular expressions in any regex-capable field. It returns one
// error per problem found (nil/empty when the config is valid).
//
// This is used by `mockly config validate` (see cmd/mockly) to let a config
// be checked without starting any servers, but is safe to call on any loaded
// Config.
func Validate(cfg *Config) []error {
	var errs []error
	errs = append(errs, validateDuplicateIDs(cfg)...)
	errs = append(errs, validateRegexes(cfg)...)
	errs = append(errs, validateBase64Fields(cfg)...)
	errs = append(errs, validateRecordConfig(cfg)...)
	errs = append(errs, validateProtocolVersions(cfg)...)
	errs = append(errs, validateAMQPTopology(cfg)...)
	errs = append(errs, validateTLSConfigs(cfg)...)
	errs = append(errs, validateHTTPListeners(cfg)...)
	return errs
}

// validateTLSConfigs checks every TLS-capable protocol's tls block:
// cert_file and key_file must either both be set or both be empty. Setting
// only one is ambiguous (ERROR) rather than silently falling back to
// certificate auto-generation, which could hide a typo'd path.
func validateTLSConfigs(cfg *Config) []error {
	var errs []error
	check := func(field string, tlsCfg *TLSConfig) {
		if tlsCfg == nil {
			return
		}
		if (tlsCfg.CertFile == "") != (tlsCfg.KeyFile == "") {
			errs = append(errs, fmt.Errorf(
				"%s: cert_file and key_file must both be set or both be empty (got cert_file=%q, key_file=%q)",
				field, tlsCfg.CertFile, tlsCfg.KeyFile,
			))
		}
	}
	check("mockly.api.tls", cfg.Mockly.API.TLS)
	if cfg.Protocols.HTTP != nil {
		check("protocols.http.tls", cfg.Protocols.HTTP.TLS)
	}
	if cfg.Protocols.WebSocket != nil {
		check("protocols.websocket.tls", cfg.Protocols.WebSocket.TLS)
	}
	if cfg.Protocols.GraphQL != nil {
		check("protocols.graphql.tls", cfg.Protocols.GraphQL.TLS)
	}
	if cfg.Protocols.TCP != nil {
		check("protocols.tcp.tls", cfg.Protocols.TCP.TLS)
	}
	if cfg.Protocols.HTTP != nil {
		for i, l := range cfg.Protocols.HTTP.Listeners {
			check(fmt.Sprintf("protocols.http.listeners[%d] (%q).tls", i, l.Name), l.TLS)
		}
	}
	return errs
}

// validateHTTPListeners checks protocols.http.listeners and the
// HTTPMock.listener references into it: every listener needs a non-empty,
// unique name and a port that doesn't collide with another listener or the
// protocol's own default port; every mock's listener (if set) must name a
// listener that's actually declared, otherwise it would silently never be
// reachable on any listener.
func validateHTTPListeners(cfg *Config) []error {
	var errs []error
	http := cfg.Protocols.HTTP
	if http == nil || len(http.Listeners) == 0 {
		return errs
	}

	names := make(map[string]bool, len(http.Listeners))
	ports := map[int]string{http.Port: "protocols.http.port"}
	for i, l := range http.Listeners {
		field := fmt.Sprintf("protocols.http.listeners[%d]", i)
		if l.Name == "" {
			errs = append(errs, fmt.Errorf("%s: name must not be empty", field))
		} else if names[l.Name] {
			errs = append(errs, fmt.Errorf("%s: duplicate listener name %q", field, l.Name))
		} else {
			names[l.Name] = true
		}
		if l.Port == 0 {
			errs = append(errs, fmt.Errorf("%s (%q): port must be set", field, l.Name))
		} else if other, ok := ports[l.Port]; ok {
			errs = append(errs, fmt.Errorf("%s (%q): port %d is already used by %s", field, l.Name, l.Port, other))
		} else {
			ports[l.Port] = fmt.Sprintf("%s (%q)", field, l.Name)
		}
	}

	for i, m := range http.Mocks {
		if m.Listener != "" && !names[m.Listener] {
			errs = append(errs, fmt.Errorf(
				"protocols.http.mocks[%d] (%q): listener %q is not declared in protocols.http.listeners",
				i, m.ID, m.Listener,
			))
		}
	}
	return errs
}

// validateAMQPTopology checks protocols.amqp.exchanges/bindings: exchange
// types must be one of the implemented values, and bindings must reference a
// non-empty exchange and queue (an empty queue would silently drop every
// message routed to it, and an empty exchange can never match a publish).
func validateAMQPTopology(cfg *Config) []error {
	var errs []error
	amqp := cfg.Protocols.AMQP
	if amqp == nil {
		return errs
	}
	validTypes := map[string]bool{"": true, "direct": true, "topic": true, "fanout": true}
	for i, ex := range amqp.Exchanges {
		if !validTypes[ex.Type] {
			errs = append(errs, fmt.Errorf(
				"protocols.amqp.exchanges[%d]: exchange %q has unsupported type %q; supported types: direct, topic, fanout",
				i, ex.Name, ex.Type,
			))
		}
	}
	for i, b := range amqp.Bindings {
		if b.Exchange == "" {
			errs = append(errs, fmt.Errorf("protocols.amqp.bindings[%d]: exchange must not be empty", i))
		}
		if b.Queue == "" {
			errs = append(errs, fmt.Errorf("protocols.amqp.bindings[%d]: queue must not be empty", i))
		}
	}
	return errs
}

// validateRecordConfig checks that HTTP record mode, when enabled, has a
// usable Target URL — an empty or malformed target would otherwise only
// surface as a confusing runtime error on the first unmatched request.
func validateRecordConfig(cfg *Config) []error {
	var errs []error
	rec := cfg.Protocols.HTTP
	if rec == nil || rec.Record == nil || !rec.Record.Enabled {
		return errs
	}
	target := strings.TrimSpace(rec.Record.Target)
	if target == "" {
		errs = append(errs, fmt.Errorf("protocols.http.record: enabled but target is empty"))
		return errs
	}
	u, err := url.Parse(target)
	if err != nil || u.Scheme == "" || u.Host == "" {
		errs = append(errs, fmt.Errorf("protocols.http.record: target %q is not a valid absolute URL", target))
	}
	return errs
}

// validateBase64Fields recursively walks the entire Config looking for
// fields literally named with a "Binary" suffix (e.g. MatchBinary,
// RespondBinary, SendBinary) and reports any whose value fails to decode as
// standard base64, since protocol servers decode these lazily on the hot
// path and would otherwise silently skip a misconfigured rule.
func validateBase64Fields(cfg *Config) []error {
	var errs []error
	walkBase64Fields(reflect.ValueOf(cfg), "", "config", &errs)
	return errs
}

func walkBase64Fields(v reflect.Value, fieldName, path string, errs *[]error) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return
		}
		walkBase64Fields(v.Elem(), fieldName, path, errs)
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" { // unexported
				continue
			}
			walkBase64Fields(v.Field(i), f.Name, path+"."+f.Name, errs)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			walkBase64Fields(v.Index(i), fieldName, fmt.Sprintf("%s[%d]", path, i), errs)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			walkBase64Fields(v.MapIndex(k), fieldName, fmt.Sprintf("%s[%v]", path, k.Interface()), errs)
		}
	case reflect.String:
		s := v.String()
		if s == "" || !strings.HasSuffix(fieldName, "Binary") {
			return
		}
		if _, err := base64.StdEncoding.DecodeString(s); err != nil {
			*errs = append(*errs, fmt.Errorf("%s: invalid base64 %q: %w", path, s, err))
		}
	}
}

// validateDuplicateIDs walks every protocol's `Mocks` slice (found generically
// via reflection, so this keeps working as new protocols are added) and
// reports an error for every mock ID that appears more than once within the
// same protocol's list.
func validateDuplicateIDs(cfg *Config) []error {
	var errs []error
	protocols := reflect.ValueOf(cfg.Protocols)
	for i := 0; i < protocols.NumField(); i++ {
		protoName := protocols.Type().Field(i).Name
		protoVal := protocols.Field(i)
		if protoVal.Kind() == reflect.Pointer {
			if protoVal.IsNil() {
				continue
			}
			protoVal = protoVal.Elem()
		}
		if protoVal.Kind() != reflect.Struct {
			continue
		}
		mocks := protoVal.FieldByName("Mocks")
		if !mocks.IsValid() || mocks.Kind() != reflect.Slice {
			continue
		}
		seen := map[string]int{}
		for j := 0; j < mocks.Len(); j++ {
			idField := mocks.Index(j).FieldByName("ID")
			if !idField.IsValid() || idField.Kind() != reflect.String {
				continue
			}
			id := idField.String()
			if id == "" {
				continue
			}
			seen[id]++
			if seen[id] == 2 {
				errs = append(errs, fmt.Errorf("protocols.%s.mocks: duplicate mock id %q", strings.ToLower(protoName), id))
			}
		}
	}
	return errs
}

// validateRegexes recursively walks the entire Config looking for
// regex-capable string fields and reports any that fail to compile:
//   - fields literally named PathRegex/URIRegex hold a raw regular
//     expression (no prefix convention);
//   - any other string field is treated as regex when it has the "re:"
//     prefix used throughout the matcher (e.g. header/query/body matchers).
func validateRegexes(cfg *Config) []error {
	var errs []error
	walkRegexFields(reflect.ValueOf(cfg), "", "config", &errs)
	return errs
}

func walkRegexFields(v reflect.Value, fieldName, path string, errs *[]error) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return
		}
		walkRegexFields(v.Elem(), fieldName, path, errs)
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" { // unexported
				continue
			}
			walkRegexFields(v.Field(i), f.Name, path+"."+f.Name, errs)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			walkRegexFields(v.Index(i), fieldName, fmt.Sprintf("%s[%d]", path, i), errs)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			walkRegexFields(v.MapIndex(k), fieldName, fmt.Sprintf("%s[%v]", path, k.Interface()), errs)
		}
	case reflect.String:
		s := v.String()
		if s == "" {
			return
		}
		switch fieldName {
		case "PathRegex", "URIRegex":
			if _, err := regexp.Compile(s); err != nil {
				*errs = append(*errs, fmt.Errorf("%s: invalid regex %q: %w", path, s, err))
			}
		default:
			if rest, ok := strings.CutPrefix(s, "re:"); ok {
				if _, err := regexp.Compile(rest); err != nil {
					*errs = append(*errs, fmt.Errorf("%s: invalid regex %q: %w", path, s, err))
				}
			}
		}
	}
}
