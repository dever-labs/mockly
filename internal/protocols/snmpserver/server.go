// Package snmpserver implements an SNMP mock agent server.
package snmpserver

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

// Server is the SNMP mock agent server.
type Server struct {
	cfg       *config.SNMPConfig
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger

	mu     sync.RWMutex
	mocks  []config.SNMPMock
	traps  []config.SNMPTrap
	values sync.Map // OID → current value (for SET support)

	engineID  string    // stable SNMPv3 engine ID for this Server instance
	startedAt time.Time // used to derive the SNMPv3 engineTime counter
	restart   chan struct{} // closed to signal a restart is needed

	ready     chan struct{} // closed once the UDP port is bound and accepting packets
	readyOnce sync.Once
}

// New creates a new SNMP Server.
func New(cfg *config.SNMPConfig, store *state.Store, sc *scenarios.Store, log *logger.Logger) *Server {
	s := &Server{
		cfg:       cfg,
		store:     store,
		scenarios: sc,
		log:       log,
		engineID:  newEngineID(),
		startedAt: time.Now(),
		restart:   make(chan struct{}),
		ready:     make(chan struct{}),
	}
	s.mocks = append([]config.SNMPMock(nil), cfg.Mocks...)
	s.traps = append([]config.SNMPTrap(nil), cfg.Traps...)
	for _, m := range s.mocks {
		s.values.Store(m.OID, m.Value)
	}
	return s
}

// newEngineID generates a random SNMPv3 AuthoritativeEngineID, stable for
// the lifetime of a Server instance. RFC 3411 requires 5-32 octets; 12
// random bytes is a convenient size that comfortably satisfies this without
// needing to encode any real enterprise/format semantics, which a mock
// agent has no use for.
func newEngineID() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf) // crypto/rand.Read never returns a short read without error
	return string(buf)
}

// Ready returns a channel that is closed once the server's UDP port has been
// bound and is accepting packets (i.e. after the first successful
// buildAndListen call in Start). Callers — chiefly tests — can wait on this
// instead of polling the port, which is unreliable for connectionless UDP
// sockets (a UDP "connect" succeeds immediately regardless of whether
// anything is listening on the other end).
func (s *Server) Ready() <-chan struct{} {
	return s.ready
}

func (s *Server) GetMocks() []config.SNMPMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.SNMPMock(nil), s.mocks...)
}

func (s *Server) SetMocks(mocks []config.SNMPMock) {
	s.mu.Lock()
	s.mocks = append([]config.SNMPMock(nil), mocks...)
	for _, m := range s.mocks {
		s.values.Store(m.OID, m.Value)
	}
	// Prepare a new restart channel before signalling.
	oldRestart := s.restart
	s.restart = make(chan struct{})
	s.mu.Unlock()

	// Signal the Start loop to rebuild. The Start loop is responsible for
	// shutting down the running server (via current.Shutdown in the
	// <-restartCh case) before re-binding the port. Calling Shutdown here
	// would race with the Start loop's own Shutdown call.
	close(oldRestart)
}

func (s *Server) GetTraps() []config.SNMPTrap {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.SNMPTrap(nil), s.traps...)
}

func (s *Server) SetTraps(traps []config.SNMPTrap) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traps = append([]config.SNMPTrap(nil), traps...)
}

// SendTrap sends a configured SNMP trap by ID.
func (s *Server) SendTrap(id string) error {
	s.mu.RLock()
	var trap *config.SNMPTrap
	for i := range s.traps {
		if s.traps[i].ID == id {
			t := s.traps[i]
			trap = &t
			break
		}
	}
	s.mu.RUnlock()

	if trap == nil {
		return fmt.Errorf("trap %q not found", id)
	}
	return s.sendTrap(trap)
}

// Start runs the SNMP agent. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	for {
		ln, ag, err := s.buildAndListen()
		if err != nil {
			return err
		}

		// Capture the restart channel that corresponds to *this* build before
		// signalling readiness. If we signalled readiness first, a caller
		// (e.g. SetMocks, invoked immediately after Ready() unblocks it) could
		// swap s.restart for a new channel before this goroutine reads it —
		// the close() meant for us would land on the old channel, which we'd
		// never observe, permanently losing the restart signal.
		s.mu.RLock()
		restartCh := s.restart
		s.mu.RUnlock()

		// The UDP port is now bound (buildAndListen calls listenUDP
		// synchronously) — signal readiness once, on the first bind.
		s.readyOnce.Do(func() { close(s.ready) })

		s.log.Log(logger.Entry{
			Protocol: "snmp",
			Method:   "START",
			Path:     fmt.Sprintf(":%d", s.cfg.Port),
		})

		errCh := make(chan error, 1)
		go func() { errCh <- s.serveForever(ln, ag) }()

		select {
		case <-ctx.Done():
			ln.Close() //nolint:errcheck
			<-errCh    // wait for serveForever to exit before returning
			return nil
		case <-restartCh:
			// SetMocks triggered a rebuild. Shut down the running listener so
			// it releases the UDP port before buildAndListen re-binds it.
			ln.Close() //nolint:errcheck
			<-errCh    // wait for serveForever to exit before re-binding the port
			continue
		case err := <-errCh:
			// If a restart was requested at the same time serveForever
			// returned, prefer the restart over treating it as a fatal error.
			select {
			case <-restartCh:
				continue
			default:
			}
			return err
		}
	}
}

// serveForever reads and dispatches UDP datagrams against ag until ln is
// closed (by Start, on shutdown or restart), at which point it returns nil —
// a closed listener is an expected, non-fatal exit, not an error.
func (s *Server) serveForever(ln *udpListener, ag *agent) error {
	buf := make([]byte, 65535)
	for {
		n, addr, err := ln.ReadFrom(buf)
		if err != nil {
			if isClosedConnError(err) {
				return nil
			}
			return fmt.Errorf("snmp server read: %w", err)
		}
		req := append([]byte(nil), buf[:n]...)
		resp, err := ag.respond(req)
		if err != nil || resp == nil {
			continue
		}
		_ = ln.WriteTo(resp, addr)
	}
}

func isClosedConnError(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection")
}

// applyProtocolVersion applies a config.SNMPConfig.ProtocolVersion pin to the
// set of USM users and the snmpV3Only flag passed to newAgent:
//   - "v3": snmpV3Only is set, so v1/v2c (community-based) requests are
//     rejected outright.
//   - "v1" or "v2c": the USM user list is cleared, so any v3 request fails
//     to find a matching user (there is no separate "v1/v2c only" flag to
//     reject v3 requests at the version-negotiation step directly).
//   - empty (default): both users and snmpV3Only pass through unchanged,
//     preserving today's behavior of accepting any dialect.
func applyProtocolVersion(version string, users []gosnmp.UsmSecurityParameters) ([]gosnmp.UsmSecurityParameters, bool) {
	switch version {
	case "v3":
		return users, true
	case "v1", "v2c":
		return nil, false
	default:
		return users, false
	}
}

func (s *Server) buildAndListen() (*udpListener, *agent, error) {
	s.mu.RLock()
	mocks := append([]config.SNMPMock(nil), s.mocks...)
	s.mu.RUnlock()

	oids := s.buildOIDs(mocks)
	users, err := s.buildUsers()
	if err != nil {
		return nil, nil, err
	}

	community := s.cfg.Community
	if community == "" {
		community = "public"
	}

	// ProtocolVersion enforcement: "v3" rejects v1/v2c requests outright
	// (snmpV3Only); "v1"/"v2c" disables v3/USM authentication entirely by
	// registering no users, so any v3 request fails to authenticate. Empty
	// (default) accepts both, unchanged from today's behavior.
	users, snmpV3Only := applyProtocolVersion(s.cfg.ProtocolVersion, users)

	engineTime := func() uint32 { return uint32(time.Since(s.startedAt).Seconds()) } //nolint:gosec // wraps harmlessly after ~136 years
	ag := newAgent(community, snmpV3Only, users, s.engineID, engineTime, oids)

	addr := fmt.Sprintf("0.0.0.0:%d", s.cfg.Port)
	ln, err := listenUDP(addr)
	if err != nil {
		return nil, nil, fmt.Errorf("snmp server listen %s: %w", addr, err)
	}
	return ln, ag, nil
}

// buildOIDs converts an SNMPMock list into pduItems for the agent's OID table.
func (s *Server) buildOIDs(mocks []config.SNMPMock) []*pduItem {
	items := make([]*pduItem, 0, len(mocks))
	for _, m := range mocks {
		mock := m // capture for closure
		asn1Type := snmpType(mock.Type)
		s.values.Store(mock.OID, mock.Value)

		item := &pduItem{
			oid:      mock.OID,
			typ:      asn1Type,
			document: mock.ID,
			onGet: func() (interface{}, error) {
				fault := s.scenarios.EffectiveSNMPFault()
				if fault != nil && fault.Delay.Duration > 0 {
					time.Sleep(fault.Delay.Duration)
				}
				if mock.State != nil {
					if val, _ := s.store.Get(mock.State.Key); val != mock.State.Value {
						return nil, fmt.Errorf("state condition not met")
					}
				}
				if fault != nil && s.scenarios.RollFault(fault.ErrorRate) {
					msg := fault.Message
					if msg == "" {
						msg = "fault injected"
					}
					return nil, fmt.Errorf("%s", msg)
				}
				raw, _ := s.values.Load(mock.OID)
				wrapped, err := wrapValue(asn1Type, raw)
				s.log.Log(logger.Entry{
					Protocol:  "snmp",
					Method:    "GET",
					Path:      mock.OID,
					MatchedID: mock.ID,
				})
				return wrapped, err
			},
			onSet: func(value interface{}) error {
				fault := s.scenarios.EffectiveSNMPFault()
				if fault != nil && fault.Delay.Duration > 0 {
					time.Sleep(fault.Delay.Duration)
				}
				if fault != nil && s.scenarios.RollFault(fault.ErrorRate) {
					msg := fault.Message
					if msg == "" {
						msg = "fault injected"
					}
					return fmt.Errorf("%s", msg)
				}
				s.values.Store(mock.OID, value)
				s.log.Log(logger.Entry{
					Protocol:  "snmp",
					Method:    "SET",
					Path:      mock.OID,
					MatchedID: mock.ID,
				})
				return nil
			},
		}
		items = append(items, item)
	}
	return items
}

// buildUsers converts config V3 users into gosnmp UsmSecurityParameters,
// deriving each user's localized auth/priv keys from its passphrase.
func (s *Server) buildUsers() ([]gosnmp.UsmSecurityParameters, error) {
	users := make([]gosnmp.UsmSecurityParameters, 0, len(s.cfg.V3Users))
	for _, u := range s.cfg.V3Users {
		usp := gosnmp.UsmSecurityParameters{
			UserName: u.Username,
		}
		if u.AuthPassphrase != "" {
			usp.AuthenticationProtocol = authProtocol(u.AuthProtocol)
			usp.AuthenticationPassphrase = u.AuthPassphrase
		}
		if u.PrivPassphrase != "" {
			usp.PrivacyProtocol = privProtocol(u.PrivProtocol)
			usp.PrivacyPassphrase = u.PrivPassphrase
		}
		if err := usp.InitSecurityKeys(); err != nil {
			return nil, fmt.Errorf("snmpv3 user %q: %w", u.Username, err)
		}
		// usp is a freshly constructed value that has never been locked, so its
		// embedded (unexported) sync.Mutex is still at its zero value here —
		// copying it via append is safe despite the govet copylocks warning.
		users = append(users, usp) //nolint:govet // see comment above
	}
	return users, nil
}

// sendTrap sends a single SNMPTrap to its configured target.
func (s *Server) sendTrap(trap *config.SNMPTrap) error {
	host, portStr, err := net.SplitHostPort(trap.Target)
	if err != nil {
		return fmt.Errorf("invalid trap target %q: %w", trap.Target, err)
	}
	port := 162
	if _, err := fmt.Sscan(portStr, &port); err != nil {
		return fmt.Errorf("invalid trap port %q: %w", portStr, err)
	}

	version := gosnmp.Version2c
	switch trap.Version {
	case "1":
		version = gosnmp.Version1
	case "3":
		version = gosnmp.Version3
	}

	community := trap.Community
	if community == "" {
		community = s.cfg.Community
	}

	g := &gosnmp.GoSNMP{
		Target:             host,
		Port:               uint16(port),
		Transport:          "udp",
		Community:          community,
		Version:            version,
		Timeout:            5 * time.Second,
		Retries:            1,
		ExponentialTimeout: false,
		MaxOids:            gosnmp.MaxOids,
	}
	if err := g.Connect(); err != nil {
		return fmt.Errorf("connecting to trap target %s: %w", trap.Target, err)
	}
	defer g.Conn.Close() //nolint:errcheck

	pdus := make([]gosnmp.SnmpPDU, 0, len(trap.Bindings))
	for _, b := range trap.Bindings {
		asn1Type := snmpType(b.Type)
		val, err := wrapValue(asn1Type, b.Value)
		if err != nil {
			return fmt.Errorf("trap binding %s: %w", b.OID, err)
		}
		pdus = append(pdus, gosnmp.SnmpPDU{
			Name:  b.OID,
			Type:  asn1Type,
			Value: val,
		})
	}

	snmpTrap := gosnmp.SnmpTrap{
		Variables: pdus,
	}

	_, err = g.SendTrap(snmpTrap)
	if err != nil {
		return fmt.Errorf("sending trap to %s: %w", trap.Target, err)
	}

	s.log.Log(logger.Entry{
		Protocol:  "snmp",
		Method:    "TRAP",
		Path:      trap.OID,
		MatchedID: trap.ID,
	})
	return nil
}

// StatusInfo returns JSON-serialisable server info.
func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	version := s.cfg.ProtocolVersion
	if version == "" {
		version = "auto (v1/v2c/v3)"
	}
	return map[string]interface{}{
		"protocol":         "snmp",
		"enabled":          s.cfg.Enabled,
		"port":             s.cfg.Port,
		"mocks":            len(s.mocks),
		"protocol_version": version,
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// snmpType maps a config type string to the gosnmp Asn1BER constant.
func snmpType(t string) gosnmp.Asn1BER {
	switch strings.ToLower(t) {
	case "integer", "int":
		return gosnmp.Integer
	case "gauge32", "gauge":
		return gosnmp.Gauge32
	case "counter32":
		return gosnmp.Counter32
	case "counter64":
		return gosnmp.Counter64
	case "timeticks":
		return gosnmp.TimeTicks
	case "ipaddress", "ip":
		return gosnmp.IPAddress
	case "objectidentifier", "oid":
		return gosnmp.ObjectIdentifier
	default: // "string", "octetstring", ""
		return gosnmp.OctetString
	}
}

// wrapValue converts a raw YAML value (string or number) to the typed value
// gosnmp expects for the given Asn1BER type. GoSNMPServer's Asn1*Wrap
// functions this used to call were all identity functions except
// Asn1IPAddressWrap (net.IP -> its string form), which is replicated below.
func wrapValue(t gosnmp.Asn1BER, raw interface{}) (interface{}, error) {
	switch t {
	case gosnmp.Integer:
		return toInt(raw), nil
	case gosnmp.Gauge32:
		return toUint(raw), nil
	case gosnmp.Counter32:
		return toUint(raw), nil
	case gosnmp.Counter64:
		return toUint64(raw), nil
	case gosnmp.TimeTicks:
		return toUint32(raw), nil
	case gosnmp.IPAddress:
		ip := net.ParseIP(fmt.Sprint(raw))
		if ip == nil {
			return net.IPv4(0, 0, 0, 0).String(), nil
		}
		return ip.String(), nil
	case gosnmp.ObjectIdentifier:
		return fmt.Sprint(raw), nil
	default: // OctetString
		// gosnmp SET PDUs deliver OctetString values as []byte — convert back to string.
		if b, ok := raw.([]byte); ok {
			return string(b), nil
		}
		return fmt.Sprint(raw), nil
	}
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case uint:
		return int(n)
	case uint32:
		return int(n)
	case uint64:
		return int(n)
	}
	var i int
	fmt.Sscan(fmt.Sprint(v), &i) //nolint:errcheck
	return i
}

func toUint(v interface{}) uint {
	switch n := v.(type) {
	case uint:
		return n
	case int:
		if n >= 0 {
			return uint(n)
		}
		return 0
	case int64:
		if n >= 0 {
			return uint(n)
		}
		return 0
	case float64:
		if n >= 0 {
			return uint(n)
		}
		return 0
	case uint32:
		return uint(n)
	case uint64:
		return uint(n)
	}
	var u uint
	fmt.Sscan(fmt.Sprint(v), &u) //nolint:errcheck
	return u
}

func toUint32(v interface{}) uint32 {
	return uint32(toUint(v))
}

func toUint64(v interface{}) uint64 {
	switch n := v.(type) {
	case uint64:
		return n
	case float64:
		if n >= 0 {
			return uint64(n)
		}
		return 0
	}
	return uint64(toUint(v))
}

// authProtocol maps a config string to a gosnmp SnmpV3AuthProtocol.
func authProtocol(p string) gosnmp.SnmpV3AuthProtocol {
	switch strings.ToLower(p) {
	case "sha":
		return gosnmp.SHA
	case "sha224":
		return gosnmp.SHA224
	case "sha256":
		return gosnmp.SHA256
	case "sha384":
		return gosnmp.SHA384
	case "sha512":
		return gosnmp.SHA512
	default: // md5 or empty
		return gosnmp.MD5
	}
}

// privProtocol maps a config string to a gosnmp SnmpV3PrivProtocol.
func privProtocol(p string) gosnmp.SnmpV3PrivProtocol {
	switch strings.ToLower(p) {
	case "aes":
		return gosnmp.AES
	case "aes192":
		return gosnmp.AES192
	case "aes256":
		return gosnmp.AES256
	default: // des or empty
		return gosnmp.DES
	}
}
