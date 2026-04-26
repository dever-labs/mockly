// Package snmpserver implements an SNMP mock agent server.
package snmpserver

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	GoSNMPServer "github.com/slayercat/GoSNMPServer"

	"github.com/gosnmp/gosnmp"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/state"
)

// Server is the SNMP mock agent server.
type Server struct {
	cfg   *config.SNMPConfig
	store *state.Store
	log   *logger.Logger

	mu     sync.RWMutex
	mocks  []config.SNMPMock
	traps  []config.SNMPTrap
	values sync.Map // OID → current value (for SET support)

	srv    *GoSNMPServer.SNMPServer
	restart chan struct{} // closed to signal a restart is needed
}

// New creates a new SNMP Server.
func New(cfg *config.SNMPConfig, store *state.Store, log *logger.Logger) *Server {
	s := &Server{
		cfg:     cfg,
		store:   store,
		log:     log,
		restart: make(chan struct{}),
	}
	s.mocks = append([]config.SNMPMock(nil), cfg.Mocks...)
	s.traps = append([]config.SNMPTrap(nil), cfg.Traps...)
	for _, m := range s.mocks {
		s.values.Store(m.OID, m.Value)
	}
	return s
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
		if err := s.buildAndListen(); err != nil {
			return err
		}

		s.log.Log(logger.Entry{
			Protocol: "snmp",
			Method:   "START",
			Path:     fmt.Sprintf(":%d", s.cfg.Port),
		})

		s.mu.RLock()
		restartCh := s.restart
		s.mu.RUnlock()

		errCh := make(chan error, 1)
		s.mu.RLock()
		current := s.srv
		s.mu.RUnlock()
		go func() { errCh <- current.ServeForever() }()

		select {
		case <-ctx.Done():
			current.Shutdown()
			return nil
		case <-restartCh:
			// SetMocks triggered a rebuild. Shut down the running server so
			// it releases the UDP port before buildAndListen re-binds it.
			current.Shutdown()
			continue
		case err := <-errCh:
			// If a restart was requested at the same time ServeForever returned,
			// prefer the restart over treating it as a fatal error.
			select {
			case <-restartCh:
				continue
			default:
			}
			return err
		}
	}
}

func (s *Server) buildAndListen() error {
	s.mu.RLock()
	mocks := append([]config.SNMPMock(nil), s.mocks...)
	s.mu.RUnlock()

	oids := s.buildOIDs(mocks)
	users := s.buildUsers()

	community := s.cfg.Community
	if community == "" {
		community = "public"
	}

	master := GoSNMPServer.MasterAgent{
		Logger: GoSNMPServer.NewDiscardLogger(),
		SecurityConfig: GoSNMPServer.SecurityConfig{
			AuthoritativeEngineBoots: 1,
			Users:                    users,
		},
		SubAgents: []*GoSNMPServer.SubAgent{
			{
				CommunityIDs: []string{community},
				OIDs:         oids,
			},
		},
	}

	srv := GoSNMPServer.NewSNMPServer(master)
	addr := fmt.Sprintf("0.0.0.0:%d", s.cfg.Port)
	if err := srv.ListenUDP("udp", addr); err != nil {
		return fmt.Errorf("snmp server listen %s: %w", addr, err)
	}
	s.mu.Lock()
	s.srv = srv
	s.mu.Unlock()
	return nil
}

// buildOIDs converts SNMPMock list into GoSNMPServer PDUValueControlItems.
func (s *Server) buildOIDs(mocks []config.SNMPMock) []*GoSNMPServer.PDUValueControlItem {
	items := make([]*GoSNMPServer.PDUValueControlItem, 0, len(mocks))
	for _, m := range mocks {
		mock := m // capture for closure
		asn1Type := snmpType(mock.Type)
		s.values.Store(mock.OID, mock.Value)

		item := &GoSNMPServer.PDUValueControlItem{
			OID:      mock.OID,
			Type:     asn1Type,
			Document: mock.ID,
			OnGet: func() (interface{}, error) {
				if mock.State != nil {
					if val, _ := s.store.Get(mock.State.Key); val != mock.State.Value {
						return nil, fmt.Errorf("state condition not met")
					}
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
			OnSet: func(value interface{}) error {
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

// buildUsers converts config V3 users into gosnmp UsmSecurityParameters.
func (s *Server) buildUsers() []gosnmp.UsmSecurityParameters {
	users := make([]gosnmp.UsmSecurityParameters, 0, len(s.cfg.V3Users))
	for _, u := range s.cfg.V3Users {
		usp := gosnmp.UsmSecurityParameters{
			UserName:                 u.Username,
			AuthenticationProtocol:  authProtocol(u.AuthProtocol),
			AuthenticationPassphrase: u.AuthPassphrase,
			PrivacyProtocol:         privProtocol(u.PrivProtocol),
			PrivacyPassphrase:       u.PrivPassphrase,
		}
		GoSNMPServer.GenKeys(&usp)
		users = append(users, usp)
	}
	return users
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
	return map[string]interface{}{
		"protocol": "snmp",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"mocks":    len(s.mocks),
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
// expected by GoSNMPServer for the given Asn1BER type.
func wrapValue(t gosnmp.Asn1BER, raw interface{}) (interface{}, error) {
	switch t {
	case gosnmp.Integer:
		return GoSNMPServer.Asn1IntegerWrap(toInt(raw)), nil
	case gosnmp.Gauge32:
		return GoSNMPServer.Asn1Gauge32Wrap(toUint(raw)), nil
	case gosnmp.Counter32:
		return GoSNMPServer.Asn1Counter32Wrap(toUint(raw)), nil
	case gosnmp.Counter64:
		return GoSNMPServer.Asn1Counter64Wrap(toUint64(raw)), nil
	case gosnmp.TimeTicks:
		return GoSNMPServer.Asn1TimeTicksWrap(toUint32(raw)), nil
	case gosnmp.IPAddress:
		ip := net.ParseIP(fmt.Sprint(raw))
		if ip == nil {
			return GoSNMPServer.Asn1IPAddressWrap(net.IPv4(0, 0, 0, 0)), nil
		}
		return GoSNMPServer.Asn1IPAddressWrap(ip), nil
	case gosnmp.ObjectIdentifier:
		return GoSNMPServer.Asn1ObjectIdentifierWrap(fmt.Sprint(raw)), nil
	default: // OctetString
		// gosnmp SET PDUs deliver OctetString values as []byte — convert back to string.
		if b, ok := raw.([]byte); ok {
			return GoSNMPServer.Asn1OctetStringWrap(string(b)), nil
		}
		return GoSNMPServer.Asn1OctetStringWrap(fmt.Sprint(raw)), nil
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
