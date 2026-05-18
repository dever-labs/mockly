package dnsserver

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
	"github.com/dever-labs/mockly/internal/state"
)

type Server struct {
	cfg       *config.DNSConfig
	store     *state.Store
	scenarios *scenarios.Store
	log       *logger.Logger

	mu    sync.RWMutex
	mocks []config.DNSMock

	udp *dns.Server
	tcp *dns.Server
}

func New(cfg *config.DNSConfig, store *state.Store, sc *scenarios.Store, log *logger.Logger) *Server {
	return &Server{cfg: cfg, store: store, scenarios: sc, log: log, mocks: append([]config.DNSMock(nil), cfg.Mocks...)}
}

func (s *Server) SetMocks(mocks []config.DNSMock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mocks = append([]config.DNSMock(nil), mocks...)
}

func (s *Server) GetMocks() []config.DNSMock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.DNSMock(nil), s.mocks...)
}

func (s *Server) Start(ctx context.Context) error {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", s.handleQuery)
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	s.udp = &dns.Server{Addr: addr, Net: "udp", Handler: mux}
	s.tcp = &dns.Server{Addr: addr, Net: "tcp", Handler: mux}

	errCh := make(chan error, 2)
	go func() { errCh <- s.udp.ListenAndServe() }()
	go func() { errCh <- s.tcp.ListenAndServe() }()

	select {
	case <-ctx.Done():
		if s.udp != nil {
			_ = s.udp.Shutdown()
		}
		if s.tcp != nil {
			_ = s.tcp.Shutdown()
		}
		return nil
	case err := <-errCh:
		if err != nil && !isDNSServerClosed(err) {
			return err
		}
		return nil
	}
}

func isDNSServerClosed(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "closed") || strings.Contains(msg, "shutdown")
}

func (s *Server) handleQuery(w dns.ResponseWriter, r *dns.Msg) {
	resp := new(dns.Msg)
	resp.SetReply(r)
	resp.Authoritative = true

	fault := s.scenarios.EffectiveDNSFault()
	if fault != nil && fault.Delay.Duration > 0 {
		time.Sleep(fault.Delay.Duration)
	}

	for _, q := range r.Question {
		mock, ok := s.matchMock(q.Name, q.Qtype)
		if !ok {
			continue
		}
		if mock.Delay.Duration > 0 {
			time.Sleep(mock.Delay.Duration)
		}
		resp.Answer = append(resp.Answer, buildRRs(q.Name, q.Qtype, mock)...)
		s.log.Log(logger.Entry{Protocol: "dns", Method: "QUERY", Path: normalizeDNSName(q.Name), Status: 0, Body: dns.TypeToString[q.Qtype], MatchedID: mock.ID})
	}

	if fault != nil && s.scenarios.RollFault(fault.ErrorRate) {
		rcode := dns.RcodeServerFailure
		switch strings.ToUpper(fault.Rcode) {
		case "NXDOMAIN":
			rcode = dns.RcodeNameError
		case "REFUSED":
			rcode = dns.RcodeRefused
		case "NOTIMP":
			rcode = dns.RcodeNotImplemented
		case "FORMERR":
			rcode = dns.RcodeFormatError
		}
		resp.Rcode = rcode
		_ = w.WriteMsg(resp)
		return
	}

	_ = w.WriteMsg(resp)
}

func (s *Server) matchMock(name string, qtype uint16) (config.DNSMock, bool) {
	typeName := strings.ToUpper(dns.TypeToString[qtype])
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mocks {
		if m.State != nil {
			if val, _ := s.store.Get(m.State.Key); val != m.State.Value {
				continue
			}
		}
		if !matchDNSName(m.Name, name) || !strings.EqualFold(m.Type, typeName) {
			continue
		}
		return m, true
	}
	return config.DNSMock{}, false
}

func matchDNSName(pattern, name string) bool {
	pattern = normalizeDNSName(pattern)
	name = normalizeDNSName(name)
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(name, suffix) && name != strings.TrimPrefix(suffix, ".")
	}
	return pattern == name
}

func normalizeDNSName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "."
	}
	if !strings.HasSuffix(name, ".") {
		name += "."
	}
	return name
}

func buildRRs(name string, qtype uint16, mock config.DNSMock) []dns.RR {
	hdr := dns.RR_Header{Name: normalizeDNSName(name), Rrtype: qtype, Class: dns.ClassINET, Ttl: mock.TTL}
	if hdr.Ttl == 0 {
		hdr.Ttl = 60
	}
	out := make([]dns.RR, 0, len(mock.Records))
	for _, record := range mock.Records {
		switch qtype {
		case dns.TypeA:
			if ip := net.ParseIP(record).To4(); ip != nil {
				out = append(out, &dns.A{Hdr: hdr, A: ip})
			}
		case dns.TypeAAAA:
			if ip := net.ParseIP(record); ip != nil {
				out = append(out, &dns.AAAA{Hdr: hdr, AAAA: ip})
			}
		case dns.TypeCNAME:
			out = append(out, &dns.CNAME{Hdr: hdr, Target: normalizeDNSName(record)})
		case dns.TypeMX:
			fields := strings.Fields(record)
			pref := uint16(10)
			target := record
			if len(fields) >= 2 {
				fmt.Sscanf(fields[0], "%d", &pref)
				target = fields[1]
			}
			out = append(out, &dns.MX{Hdr: hdr, Preference: pref, Mx: normalizeDNSName(target)})
		case dns.TypeTXT:
			out = append(out, &dns.TXT{Hdr: hdr, Txt: []string{record}})
		case dns.TypeNS:
			out = append(out, &dns.NS{Hdr: hdr, Ns: normalizeDNSName(record)})
		case dns.TypeSRV:
			fields := strings.Fields(record)
			priority, weight, port := uint16(10), uint16(5), uint16(0)
			target := record
			if len(fields) >= 4 {
				fmt.Sscanf(fields[0], "%d", &priority)
				fmt.Sscanf(fields[1], "%d", &weight)
				fmt.Sscanf(fields[2], "%d", &port)
				target = fields[3]
			}
			out = append(out, &dns.SRV{Hdr: hdr, Priority: priority, Weight: weight, Port: port, Target: normalizeDNSName(target)})
		}
	}
	return out
}

func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{"protocol": "dns", "enabled": s.cfg.Enabled, "port": s.cfg.Port, "mocks": len(s.mocks)}
}
