package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/protocols/amqpserver"
	"github.com/dever-labs/mockly/internal/protocols/kafkaserver"
	"github.com/dever-labs/mockly/internal/protocols/stompserver"
)

type DNSProtocol interface {
	ProtocolServer
	GetMocks() []config.DNSMock
	SetMocks([]config.DNSMock)
}

type AMQPProtocol interface {
	ProtocolServer
	GetMocks() []config.AMQPMock
	SetMocks([]config.AMQPMock)
	GetMessageStore() *amqpserver.MessageStore
}

type KafkaProtocol interface {
	ProtocolServer
	GetMocks() []config.KafkaMock
	SetMocks([]config.KafkaMock)
	GetMessageStore() *kafkaserver.MessageStore
}

type LDAPProtocol interface {
	ProtocolServer
	GetMocks() []config.LDAPMock
	SetMocks([]config.LDAPMock)
}

type IMAPProtocol interface {
	ProtocolServer
	GetMailboxes() []config.IMAPMailbox
	SetMailboxes([]config.IMAPMailbox)
}

type FTPProtocol interface {
	ProtocolServer
	GetFiles() []config.FTPFile
	SetFiles([]config.FTPFile)
}

type MemcachedProtocol interface {
	ProtocolServer
	GetMocks() []config.MemcachedMock
	SetMocks([]config.MemcachedMock)
}

type STOMPProtocol interface {
	ProtocolServer
	GetMocks() []config.STOMPMock
	SetMocks([]config.STOMPMock)
	GetMessageStore() *stompserver.MessageStore
}

type CoAPProtocol interface {
	ProtocolServer
	GetMocks() []config.CoAPMock
	SetMocks([]config.CoAPMock)
}

type SIPProtocol interface {
	ProtocolServer
	GetMocks() []config.SIPMock
	SetMocks([]config.SIPMock)
}

func (s *Server) listDNSMocks(w http.ResponseWriter, r *http.Request) {
	if s.dns == nil {
		writeJSON(w, http.StatusOK, []config.DNSMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.dns.GetMocks())
}

func (s *Server) addDNSMock(w http.ResponseWriter, r *http.Request) {
	if s.dns == nil {
		writeError(w, http.StatusServiceUnavailable, "dns protocol not enabled")
		return
	}
	var m config.DNSMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("dns-%d", time.Now().UnixNano())
	}
	s.dns.SetMocks(append(s.dns.GetMocks(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateDNSMock(w http.ResponseWriter, r *http.Request) {
	if s.dns == nil {
		writeError(w, http.StatusServiceUnavailable, "dns protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	var updated config.DNSMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.dns.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.dns.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteDNSMock(w http.ResponseWriter, r *http.Request) {
	if s.dns == nil {
		writeError(w, http.StatusServiceUnavailable, "dns protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	mocks := s.dns.GetMocks()
	filtered := make([]config.DNSMock, 0, len(mocks))
	found := false
	for _, m := range mocks {
		if m.ID == id {
			found = true
		} else {
			filtered = append(filtered, m)
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "mock not found")
		return
	}
	s.dns.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) listAMQPMocks(w http.ResponseWriter, r *http.Request) {
	if s.amqp == nil {
		writeJSON(w, http.StatusOK, []config.AMQPMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.amqp.GetMocks())
}

func (s *Server) addAMQPMock(w http.ResponseWriter, r *http.Request) {
	if s.amqp == nil {
		writeError(w, http.StatusServiceUnavailable, "amqp protocol not enabled")
		return
	}
	var m config.AMQPMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("amqp-%d", time.Now().UnixNano())
	}
	s.amqp.SetMocks(append(s.amqp.GetMocks(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateAMQPMock(w http.ResponseWriter, r *http.Request) {
	if s.amqp == nil {
		writeError(w, http.StatusServiceUnavailable, "amqp protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	var updated config.AMQPMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.amqp.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.amqp.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteAMQPMock(w http.ResponseWriter, r *http.Request) {
	if s.amqp == nil {
		writeError(w, http.StatusServiceUnavailable, "amqp protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	mocks := s.amqp.GetMocks()
	filtered := make([]config.AMQPMock, 0, len(mocks))
	found := false
	for _, m := range mocks {
		if m.ID == id {
			found = true
		} else {
			filtered = append(filtered, m)
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "mock not found")
		return
	}
	s.amqp.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) listAMQPMessages(w http.ResponseWriter, r *http.Request) {
	if s.amqp == nil {
		writeJSON(w, http.StatusOK, []config.ReceivedAMQPMessage{})
		return
	}
	writeJSON(w, http.StatusOK, s.amqp.GetMessageStore().All())
}

func (s *Server) clearAMQPMessages(w http.ResponseWriter, r *http.Request) {
	if s.amqp != nil {
		s.amqp.GetMessageStore().Clear()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (s *Server) listKafkaMocks(w http.ResponseWriter, r *http.Request) {
	if s.kafka == nil {
		writeJSON(w, http.StatusOK, []config.KafkaMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.kafka.GetMocks())
}

func (s *Server) addKafkaMock(w http.ResponseWriter, r *http.Request) {
	if s.kafka == nil {
		writeError(w, http.StatusServiceUnavailable, "kafka protocol not enabled")
		return
	}
	var m config.KafkaMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("kafka-%d", time.Now().UnixNano())
	}
	s.kafka.SetMocks(append(s.kafka.GetMocks(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateKafkaMock(w http.ResponseWriter, r *http.Request) {
	if s.kafka == nil {
		writeError(w, http.StatusServiceUnavailable, "kafka protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	var updated config.KafkaMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.kafka.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.kafka.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteKafkaMock(w http.ResponseWriter, r *http.Request) {
	if s.kafka == nil {
		writeError(w, http.StatusServiceUnavailable, "kafka protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	mocks := s.kafka.GetMocks()
	filtered := make([]config.KafkaMock, 0, len(mocks))
	found := false
	for _, m := range mocks {
		if m.ID == id {
			found = true
		} else {
			filtered = append(filtered, m)
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "mock not found")
		return
	}
	s.kafka.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) listKafkaMessages(w http.ResponseWriter, r *http.Request) {
	if s.kafka == nil {
		writeJSON(w, http.StatusOK, []config.ProducedKafkaMessage{})
		return
	}
	writeJSON(w, http.StatusOK, s.kafka.GetMessageStore().All())
}

func (s *Server) clearKafkaMessages(w http.ResponseWriter, r *http.Request) {
	if s.kafka != nil {
		s.kafka.GetMessageStore().Clear()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (s *Server) listLDAPMocks(w http.ResponseWriter, r *http.Request) {
	if s.ldap == nil {
		writeJSON(w, http.StatusOK, []config.LDAPMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.ldap.GetMocks())
}

func (s *Server) addLDAPMock(w http.ResponseWriter, r *http.Request) {
	if s.ldap == nil {
		writeError(w, http.StatusServiceUnavailable, "ldap protocol not enabled")
		return
	}
	var m config.LDAPMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("ldap-%d", time.Now().UnixNano())
	}
	s.ldap.SetMocks(append(s.ldap.GetMocks(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateLDAPMock(w http.ResponseWriter, r *http.Request) {
	if s.ldap == nil {
		writeError(w, http.StatusServiceUnavailable, "ldap protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	var updated config.LDAPMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.ldap.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.ldap.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteLDAPMock(w http.ResponseWriter, r *http.Request) {
	if s.ldap == nil {
		writeError(w, http.StatusServiceUnavailable, "ldap protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	mocks := s.ldap.GetMocks()
	filtered := make([]config.LDAPMock, 0, len(mocks))
	found := false
	for _, m := range mocks {
		if m.ID == id {
			found = true
		} else {
			filtered = append(filtered, m)
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "mock not found")
		return
	}
	s.ldap.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) listIMAPMailboxes(w http.ResponseWriter, r *http.Request) {
	if s.imap == nil {
		writeJSON(w, http.StatusOK, []config.IMAPMailbox{})
		return
	}
	writeJSON(w, http.StatusOK, s.imap.GetMailboxes())
}

func (s *Server) addIMAPMailbox(w http.ResponseWriter, r *http.Request) {
	if s.imap == nil {
		writeError(w, http.StatusServiceUnavailable, "imap protocol not enabled")
		return
	}
	var m config.IMAPMailbox
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("imap-%d", time.Now().UnixNano())
	}
	s.imap.SetMailboxes(append(s.imap.GetMailboxes(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateIMAPMailbox(w http.ResponseWriter, r *http.Request) {
	if s.imap == nil {
		writeError(w, http.StatusServiceUnavailable, "imap protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	var updated config.IMAPMailbox
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	items := s.imap.GetMailboxes()
	for i, m := range items {
		if m.ID == id {
			items[i] = updated
			s.imap.SetMailboxes(items)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteIMAPMailbox(w http.ResponseWriter, r *http.Request) {
	if s.imap == nil {
		writeError(w, http.StatusServiceUnavailable, "imap protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	items := s.imap.GetMailboxes()
	filtered := make([]config.IMAPMailbox, 0, len(items))
	found := false
	for _, m := range items {
		if m.ID == id {
			found = true
		} else {
			filtered = append(filtered, m)
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "mock not found")
		return
	}
	s.imap.SetMailboxes(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) listFTPFiles(w http.ResponseWriter, r *http.Request) {
	if s.ftp == nil {
		writeJSON(w, http.StatusOK, []config.FTPFile{})
		return
	}
	writeJSON(w, http.StatusOK, s.ftp.GetFiles())
}

func (s *Server) addFTPFile(w http.ResponseWriter, r *http.Request) {
	if s.ftp == nil {
		writeError(w, http.StatusServiceUnavailable, "ftp protocol not enabled")
		return
	}
	var m config.FTPFile
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("ftp-%d", time.Now().UnixNano())
	}
	s.ftp.SetFiles(append(s.ftp.GetFiles(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateFTPFile(w http.ResponseWriter, r *http.Request) {
	if s.ftp == nil {
		writeError(w, http.StatusServiceUnavailable, "ftp protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	var updated config.FTPFile
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	items := s.ftp.GetFiles()
	for i, m := range items {
		if m.ID == id {
			items[i] = updated
			s.ftp.SetFiles(items)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteFTPFile(w http.ResponseWriter, r *http.Request) {
	if s.ftp == nil {
		writeError(w, http.StatusServiceUnavailable, "ftp protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	items := s.ftp.GetFiles()
	filtered := make([]config.FTPFile, 0, len(items))
	found := false
	for _, m := range items {
		if m.ID == id {
			found = true
		} else {
			filtered = append(filtered, m)
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "mock not found")
		return
	}
	s.ftp.SetFiles(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) listMemcachedMocks(w http.ResponseWriter, r *http.Request) {
	if s.memcached == nil {
		writeJSON(w, http.StatusOK, []config.MemcachedMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.memcached.GetMocks())
}

func (s *Server) addMemcachedMock(w http.ResponseWriter, r *http.Request) {
	if s.memcached == nil {
		writeError(w, http.StatusServiceUnavailable, "memcached protocol not enabled")
		return
	}
	var m config.MemcachedMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("memcached-%d", time.Now().UnixNano())
	}
	s.memcached.SetMocks(append(s.memcached.GetMocks(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateMemcachedMock(w http.ResponseWriter, r *http.Request) {
	if s.memcached == nil {
		writeError(w, http.StatusServiceUnavailable, "memcached protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	var updated config.MemcachedMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.memcached.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.memcached.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteMemcachedMock(w http.ResponseWriter, r *http.Request) {
	if s.memcached == nil {
		writeError(w, http.StatusServiceUnavailable, "memcached protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	mocks := s.memcached.GetMocks()
	filtered := make([]config.MemcachedMock, 0, len(mocks))
	found := false
	for _, m := range mocks {
		if m.ID == id {
			found = true
		} else {
			filtered = append(filtered, m)
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "mock not found")
		return
	}
	s.memcached.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) listSTOMPMocks(w http.ResponseWriter, r *http.Request) {
	if s.stomp == nil {
		writeJSON(w, http.StatusOK, []config.STOMPMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.stomp.GetMocks())
}

func (s *Server) addSTOMPMock(w http.ResponseWriter, r *http.Request) {
	if s.stomp == nil {
		writeError(w, http.StatusServiceUnavailable, "stomp protocol not enabled")
		return
	}
	var m config.STOMPMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("stomp-%d", time.Now().UnixNano())
	}
	s.stomp.SetMocks(append(s.stomp.GetMocks(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateSTOMPMock(w http.ResponseWriter, r *http.Request) {
	if s.stomp == nil {
		writeError(w, http.StatusServiceUnavailable, "stomp protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	var updated config.STOMPMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.stomp.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.stomp.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteSTOMPMock(w http.ResponseWriter, r *http.Request) {
	if s.stomp == nil {
		writeError(w, http.StatusServiceUnavailable, "stomp protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	mocks := s.stomp.GetMocks()
	filtered := make([]config.STOMPMock, 0, len(mocks))
	found := false
	for _, m := range mocks {
		if m.ID == id {
			found = true
		} else {
			filtered = append(filtered, m)
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "mock not found")
		return
	}
	s.stomp.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) listSTOMPMessages(w http.ResponseWriter, r *http.Request) {
	if s.stomp == nil {
		writeJSON(w, http.StatusOK, []config.ReceivedSTOMPMessage{})
		return
	}
	writeJSON(w, http.StatusOK, s.stomp.GetMessageStore().All())
}

func (s *Server) clearSTOMPMessages(w http.ResponseWriter, r *http.Request) {
	if s.stomp != nil {
		s.stomp.GetMessageStore().Clear()
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (s *Server) listCoAPMocks(w http.ResponseWriter, r *http.Request) {
	if s.coap == nil {
		writeJSON(w, http.StatusOK, []config.CoAPMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.coap.GetMocks())
}

func (s *Server) addCoAPMock(w http.ResponseWriter, r *http.Request) {
	if s.coap == nil {
		writeError(w, http.StatusServiceUnavailable, "coap protocol not enabled")
		return
	}
	var m config.CoAPMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("coap-%d", time.Now().UnixNano())
	}
	s.coap.SetMocks(append(s.coap.GetMocks(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateCoAPMock(w http.ResponseWriter, r *http.Request) {
	if s.coap == nil {
		writeError(w, http.StatusServiceUnavailable, "coap protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	var updated config.CoAPMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.coap.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.coap.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteCoAPMock(w http.ResponseWriter, r *http.Request) {
	if s.coap == nil {
		writeError(w, http.StatusServiceUnavailable, "coap protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	mocks := s.coap.GetMocks()
	filtered := make([]config.CoAPMock, 0, len(mocks))
	found := false
	for _, m := range mocks {
		if m.ID == id {
			found = true
		} else {
			filtered = append(filtered, m)
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "mock not found")
		return
	}
	s.coap.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}

func (s *Server) listSIPMocks(w http.ResponseWriter, r *http.Request) {
	if s.sip == nil {
		writeJSON(w, http.StatusOK, []config.SIPMock{})
		return
	}
	writeJSON(w, http.StatusOK, s.sip.GetMocks())
}

func (s *Server) addSIPMock(w http.ResponseWriter, r *http.Request) {
	if s.sip == nil {
		writeError(w, http.StatusServiceUnavailable, "sip protocol not enabled")
		return
	}
	var m config.SIPMock
	if err := decodeBody(r, &m); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = fmt.Sprintf("sip-%d", time.Now().UnixNano())
	}
	s.sip.SetMocks(append(s.sip.GetMocks(), m))
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) updateSIPMock(w http.ResponseWriter, r *http.Request) {
	if s.sip == nil {
		writeError(w, http.StatusServiceUnavailable, "sip protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	var updated config.SIPMock
	if err := decodeBody(r, &updated); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated.ID = id
	mocks := s.sip.GetMocks()
	for i, m := range mocks {
		if m.ID == id {
			mocks[i] = updated
			s.sip.SetMocks(mocks)
			writeJSON(w, http.StatusOK, updated)
			return
		}
	}
	writeError(w, http.StatusNotFound, "mock not found")
}

func (s *Server) deleteSIPMock(w http.ResponseWriter, r *http.Request) {
	if s.sip == nil {
		writeError(w, http.StatusServiceUnavailable, "sip protocol not enabled")
		return
	}
	id := chi.URLParam(r, "id")
	mocks := s.sip.GetMocks()
	filtered := make([]config.SIPMock, 0, len(mocks))
	found := false
	for _, m := range mocks {
		if m.ID == id {
			found = true
		} else {
			filtered = append(filtered, m)
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "mock not found")
		return
	}
	s.sip.SetMocks(filtered)
	writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
}
