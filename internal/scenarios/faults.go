package scenarios

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dever-labs/mockly/internal/config"
)

func effectiveFault[T any](s *Store, selector func(*config.ProtocolFaults) *T) *T {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var (
		result   *T
		winnerID string
	)
	for id := range s.active {
		if sc, ok := s.scenarios[id]; ok && sc.Faults != nil {
			if fault := selector(sc.Faults); fault != nil {
				if result == nil || id > winnerID {
					result = fault
					winnerID = id
				}
			}
		}
	}
	if result != nil {
		return result
	}
	return selector(&s.direct)
}

func (s *Store) EffectiveDNSFault() *config.DNSFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.DNSFault { return f.DNS })
}

func (s *Store) EffectiveGRPCFault() *config.GRPCFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.GRPCFault { return f.GRPC })
}

func (s *Store) EffectiveHTTPFault() *config.HTTPFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.HTTPFault { return f.HTTP })
}

func (s *Store) EffectiveGraphQLFault() *config.HTTPFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.HTTPFault { return f.GraphQL })
}

func (s *Store) EffectiveWebSocketFault() *config.WebSocketFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.WebSocketFault { return f.WebSocket })
}

func (s *Store) EffectiveTCPFault() *config.TCPFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.TCPFault { return f.TCP })
}

func (s *Store) EffectiveRedisFault() *config.RedisFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.RedisFault { return f.Redis })
}

func (s *Store) EffectiveMQTTFault() *config.MQTTFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.MQTTFault { return f.MQTT })
}

func (s *Store) EffectiveSMTPFault() *config.SMTPFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.SMTPFault { return f.SMTP })
}

func (s *Store) EffectiveSNMPFault() *config.SNMPFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.SNMPFault { return f.SNMP })
}

func (s *Store) EffectiveAMQPFault() *config.AMQPFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.AMQPFault { return f.AMQP })
}

func (s *Store) EffectiveKafkaFault() *config.KafkaFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.KafkaFault { return f.Kafka })
}

func (s *Store) EffectiveLDAPFault() *config.LDAPFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.LDAPFault { return f.LDAP })
}

func (s *Store) EffectiveIMAPFault() *config.IMAPFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.IMAPFault { return f.IMAP })
}

func (s *Store) EffectiveFTPFault() *config.FTPFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.FTPFault { return f.FTP })
}

func (s *Store) EffectiveMemcachedFault() *config.MemcachedFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.MemcachedFault { return f.Memcached })
}

func (s *Store) EffectiveSTOMPFault() *config.STOMPFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.STOMPFault { return f.STOMP })
}

func (s *Store) EffectiveCoAPFault() *config.CoAPFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.CoAPFault { return f.CoAP })
}

func (s *Store) EffectiveSIPFault() *config.SIPFault {
	return effectiveFault(s, func(f *config.ProtocolFaults) *config.SIPFault { return f.SIP })
}

// GetDirectProtocolFault returns the direct fault for the named protocol, or nil.
func (s *Store) GetDirectProtocolFault(protocol string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	switch strings.ToLower(protocol) {
	case "http":
		return s.direct.HTTP
	case "graphql":
		return s.direct.GraphQL
	case "websocket":
		return s.direct.WebSocket
	case "grpc":
		return s.direct.GRPC
	case "tcp":
		return s.direct.TCP
	case "redis":
		return s.direct.Redis
	case "mqtt":
		return s.direct.MQTT
	case "smtp":
		return s.direct.SMTP
	case "snmp":
		return s.direct.SNMP
	case "dns":
		return s.direct.DNS
	case "amqp":
		return s.direct.AMQP
	case "kafka":
		return s.direct.Kafka
	case "ldap":
		return s.direct.LDAP
	case "imap":
		return s.direct.IMAP
	case "ftp":
		return s.direct.FTP
	case "memcached":
		return s.direct.Memcached
	case "stomp":
		return s.direct.STOMP
	case "coap":
		return s.direct.CoAP
	case "sip":
		return s.direct.SIP
	}
	return nil
}

// SetDirectProtocolFaultJSON unmarshals JSON into the right fault type and sets it.
func (s *Store) SetDirectProtocolFaultJSON(protocol string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch strings.ToLower(protocol) {
	case "http":
		{
			var f config.HTTPFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.HTTP = &f
		}
	case "graphql":
		{
			var f config.HTTPFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.GraphQL = &f
		}
	case "websocket":
		{
			var f config.WebSocketFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.WebSocket = &f
		}
	case "grpc":
		{
			var f config.GRPCFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.GRPC = &f
		}
	case "tcp":
		{
			var f config.TCPFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.TCP = &f
		}
	case "redis":
		{
			var f config.RedisFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.Redis = &f
		}
	case "mqtt":
		{
			var f config.MQTTFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.MQTT = &f
		}
	case "smtp":
		{
			var f config.SMTPFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.SMTP = &f
		}
	case "snmp":
		{
			var f config.SNMPFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.SNMP = &f
		}
	case "dns":
		{
			var f config.DNSFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.DNS = &f
		}
	case "amqp":
		{
			var f config.AMQPFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.AMQP = &f
		}
	case "kafka":
		{
			var f config.KafkaFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.Kafka = &f
		}
	case "ldap":
		{
			var f config.LDAPFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.LDAP = &f
		}
	case "imap":
		{
			var f config.IMAPFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.IMAP = &f
		}
	case "ftp":
		{
			var f config.FTPFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.FTP = &f
		}
	case "memcached":
		{
			var f config.MemcachedFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.Memcached = &f
		}
	case "stomp":
		{
			var f config.STOMPFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.STOMP = &f
		}
	case "coap":
		{
			var f config.CoAPFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.CoAP = &f
		}
	case "sip":
		{
			var f config.SIPFault
			if err := json.Unmarshal(data, &f); err != nil {
				return err
			}
			s.direct.SIP = &f
		}
	default:
		return fmt.Errorf("unknown protocol %q", protocol)
	}
	return nil
}

// ClearDirectProtocolFault removes the direct fault for the named protocol.
func (s *Store) ClearDirectProtocolFault(protocol string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch strings.ToLower(protocol) {
	case "http":
		s.direct.HTTP = nil
	case "graphql":
		s.direct.GraphQL = nil
	case "websocket":
		s.direct.WebSocket = nil
	case "grpc":
		s.direct.GRPC = nil
	case "tcp":
		s.direct.TCP = nil
	case "redis":
		s.direct.Redis = nil
	case "mqtt":
		s.direct.MQTT = nil
	case "smtp":
		s.direct.SMTP = nil
	case "snmp":
		s.direct.SNMP = nil
	case "dns":
		s.direct.DNS = nil
	case "amqp":
		s.direct.AMQP = nil
	case "kafka":
		s.direct.Kafka = nil
	case "ldap":
		s.direct.LDAP = nil
	case "imap":
		s.direct.IMAP = nil
	case "ftp":
		s.direct.FTP = nil
	case "memcached":
		s.direct.Memcached = nil
	case "stomp":
		s.direct.STOMP = nil
	case "coap":
		s.direct.CoAP = nil
	case "sip":
		s.direct.SIP = nil
	}
}
