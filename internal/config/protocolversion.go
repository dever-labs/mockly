package config

import (
	"fmt"
	"strings"
)

// supportedProtocolVersions maps each protocol that supports version pinning
// to the set of ProtocolVersion values accepted by the running binary. An
// empty ProtocolVersion is always valid for every protocol (it selects
// today's default behavior) and is intentionally not listed here.
//
// Values present in this map but not yet backed by a real wire-level
// implementation (e.g. AMQP 1.0, LDAPv2) are called out in the per-field doc
// comments on the corresponding Config struct and deliberately omitted here,
// so attempting to select them fails validation with a clear error instead
// of silently starting a server that cannot speak that dialect.
var supportedProtocolVersions = map[string][]string{
	"mqtt": {"3.1", "3.1.1", "5.0"},
	"amqp": {"0.9.1"},
	"snmp": {"v1", "v2c", "v3"},
	"ldap": {"v3"},
}

// validateProtocolVersions checks each protocol's ProtocolVersion field
// against the list of versions this binary actually supports for that
// protocol, reporting a clear error for any unrecognized or unsupported
// value. New protocols must be wired in explicitly below (there is no
// reflection); adding an entry to supportedProtocolVersions alone is not
// sufficient.
func validateProtocolVersions(cfg *Config) []error {
	var errs []error

	check := func(proto, version string) {
		if version == "" {
			return
		}
		supported, ok := supportedProtocolVersions[proto]
		if !ok {
			return
		}
		for _, v := range supported {
			if v == version {
				return
			}
		}
		errs = append(errs, fmt.Errorf(
			"protocols.%s.protocol_version: %q is not supported by this build; supported versions: %s",
			proto, version, strings.Join(supported, ", "),
		))
	}

	if cfg.Protocols.MQTT != nil {
		check("mqtt", cfg.Protocols.MQTT.ProtocolVersion)
	}
	if cfg.Protocols.AMQP != nil {
		check("amqp", cfg.Protocols.AMQP.ProtocolVersion)
	}
	if cfg.Protocols.SNMP != nil {
		check("snmp", cfg.Protocols.SNMP.ProtocolVersion)
	}
	if cfg.Protocols.LDAP != nil {
		check("ldap", cfg.Protocols.LDAP.ProtocolVersion)
	}

	return errs
}
