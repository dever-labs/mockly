package imapserver

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
	"github.com/dever-labs/mockly/internal/scenarios"
)

type Server struct {
	cfg       *config.IMAPConfig
	scenarios *scenarios.Store
	log       *logger.Logger

	mu        sync.RWMutex
	mailboxes []config.IMAPMailbox
	listener  net.Listener
}

func New(cfg *config.IMAPConfig, sc *scenarios.Store, log *logger.Logger) *Server {
	return &Server{cfg: cfg, scenarios: sc, log: log, mailboxes: append([]config.IMAPMailbox(nil), cfg.Mailboxes...)}
}

func (s *Server) SetMailboxes(mailboxes []config.IMAPMailbox) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mailboxes = append([]config.IMAPMailbox(nil), mailboxes...)
}

func (s *Server) GetMailboxes() []config.IMAPMailbox {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]config.IMAPMailbox(nil), s.mailboxes...)
}

func (s *Server) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.Port))
	if err != nil {
		return fmt.Errorf("imap server listen :%d: %w", s.cfg.Port, err)
	}
	s.listener = ln
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close() //nolint:errcheck
	reader := bufio.NewReader(conn)
	state := "NOT_AUTHENTICATED"
	selected := config.IMAPMailbox{}
	_, _ = conn.Write([]byte("* OK IMAP4rev1 Mockly IMAP Server ready\r\n"))
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		tag, cmd := parts[0], strings.ToUpper(parts[1])
		args := parts[2:]
		switch cmd {
		case "CAPABILITY":
			_, _ = fmt.Fprintf(conn, "* CAPABILITY IMAP4rev1\r\n%s OK CAPABILITY completed\r\n", tag)
		case "NOOP":
			_, _ = fmt.Fprintf(conn, "%s OK NOOP completed\r\n", tag)
		case "LOGIN":
			if s.authenticate(args) {
				state = "AUTHENTICATED"
				_, _ = fmt.Fprintf(conn, "%s OK LOGIN completed\r\n", tag)
			} else {
				_, _ = fmt.Fprintf(conn, "%s NO LOGIN failed\r\n", tag)
			}
		case "LIST":
			for _, mb := range s.GetMailboxes() {
				_, _ = fmt.Fprintf(conn, "* LIST (\\HasNoChildren) \"/\" %s\r\n", quoteMailbox(mb.Name))
			}
			_, _ = fmt.Fprintf(conn, "%s OK LIST completed\r\n", tag)
		case "SELECT":
			if state == "NOT_AUTHENTICATED" {
				_, _ = fmt.Fprintf(conn, "%s NO Authenticate first\r\n", tag)
				continue
			}
			mb, ok := s.findMailbox(strings.Join(args, " "))
			if !ok {
				_, _ = fmt.Fprintf(conn, "%s NO Mailbox not found\r\n", tag)
				continue
			}
			selected = mb
			state = "SELECTED"
			_, _ = fmt.Fprintf(conn, "* %d EXISTS\r\n* 0 RECENT\r\n* OK [UNSEEN 1]\r\n* OK [UIDVALIDITY 1]\r\n* FLAGS (\\Answered \\Flagged \\Deleted \\Seen \\Draft)\r\n%s OK [READ-WRITE] SELECT completed\r\n", len(mb.Messages), tag)
		case "FETCH":
			if state != "SELECTED" {
				_, _ = fmt.Fprintf(conn, "%s NO Select mailbox first\r\n", tag)
				continue
			}
			fault := s.scenarios.EffectiveIMAPFault()
			if fault != nil && fault.Delay.Duration > 0 {
				time.Sleep(fault.Delay.Duration)
			}
			if fault != nil && s.scenarios.RollFault(fault.ErrorRate) {
				resp := fault.Response
				if resp == "" {
					resp = "NO"
				}
				msg := fault.Message
				if msg == "" {
					msg = "fault injected"
				}
				_, _ = fmt.Fprintf(conn, "%s %s %s\r\n", tag, resp, msg)
				continue
			}
			for _, msg := range fetchMessages(selected.Messages, firstArg(args)) {
				resp := buildFetchResponse(msg, strings.Join(args[1:], " "))
				_, _ = conn.Write([]byte(resp))
			}
			_, _ = fmt.Fprintf(conn, "%s OK FETCH completed\r\n", tag)
		case "SEARCH":
			if state != "SELECTED" {
				_, _ = fmt.Fprintf(conn, "%s NO Select mailbox first\r\n", tag)
				continue
			}
			fault := s.scenarios.EffectiveIMAPFault()
			if fault != nil && fault.Delay.Duration > 0 {
				time.Sleep(fault.Delay.Duration)
			}
			if fault != nil && s.scenarios.RollFault(fault.ErrorRate) {
				resp := fault.Response
				if resp == "" {
					resp = "NO"
				}
				msg := fault.Message
				if msg == "" {
					msg = "fault injected"
				}
				_, _ = fmt.Fprintf(conn, "%s %s %s\r\n", tag, resp, msg)
				continue
			}
			seqs := make([]string, 0, len(selected.Messages))
			for _, msg := range selected.Messages {
				seqs = append(seqs, strconv.Itoa(msg.SeqNum))
			}
			_, _ = fmt.Fprintf(conn, "* SEARCH %s\r\n%s OK SEARCH completed\r\n", strings.Join(seqs, " "), tag)
		case "LOGOUT":
			_, _ = fmt.Fprintf(conn, "* BYE IMAP4rev1 Server logging out\r\n%s OK LOGOUT completed\r\n", tag)
			return
		default:
			_, _ = fmt.Fprintf(conn, "%s BAD Unknown command\r\n", tag)
		}
	}
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func (s *Server) authenticate(args []string) bool {
	if len(s.cfg.Users) == 0 {
		return true
	}
	if len(args) < 2 {
		return false
	}
	user := strings.Trim(args[0], `"`)
	pass := strings.Trim(args[1], `"`)
	for _, u := range s.cfg.Users {
		if u.Username == user && u.Password == pass {
			return true
		}
	}
	return false
}

func quoteMailbox(name string) string {
	if strings.HasPrefix(name, `"`) {
		return name
	}
	return fmt.Sprintf(`"%s"`, name)
}

func (s *Server) findMailbox(name string) (config.IMAPMailbox, bool) {
	name = strings.Trim(name, `"`)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, mb := range s.mailboxes {
		if strings.EqualFold(mb.Name, name) {
			return mb, true
		}
	}
	return config.IMAPMailbox{}, false
}

func fetchMessages(messages []config.IMAPMessage, seq string) []config.IMAPMessage {
	if seq == "1:*" || seq == "*" || seq == "" {
		return append([]config.IMAPMessage(nil), messages...)
	}
	want, _ := strconv.Atoi(seq)
	for _, msg := range messages {
		if msg.SeqNum == want {
			return []config.IMAPMessage{msg}
		}
	}
	return nil
}

func buildFetchResponse(msg config.IMAPMessage, items string) string {
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\n", msg.From, msg.To, msg.Subject, msg.Date)
	full := headers + "\r\n" + msg.Body
	parts := make([]string, 0, 4)
	items = strings.ToUpper(items)
	if strings.Contains(items, "FLAGS") {
		parts = append(parts, fmt.Sprintf("FLAGS (%s)", strings.Join(msg.Flags, " ")))
	}
	if strings.Contains(items, "RFC822.SIZE") {
		parts = append(parts, fmt.Sprintf("RFC822.SIZE %d", len(full)))
	}
	if strings.Contains(items, "BODY[HEADER.FIELDS") {
		parts = append(parts, fmt.Sprintf("BODY[HEADER.FIELDS (FROM TO SUBJECT DATE)] {%d}\r\n%s", len(headers), headers))
	}
	if strings.Contains(items, "BODY[]") || strings.Contains(items, "RFC822") {
		parts = append(parts, fmt.Sprintf("BODY[] {%d}\r\n%s", len(full), full))
	} else if strings.Contains(items, "BODY[TEXT]") {
		parts = append(parts, fmt.Sprintf("BODY[TEXT] {%d}\r\n%s", len(msg.Body), msg.Body))
	}
	return fmt.Sprintf("* %d FETCH (%s)\r\n", msg.SeqNum, strings.Join(parts, " "))
}

func (s *Server) StatusInfo() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]interface{}{"protocol": "imap", "enabled": s.cfg.Enabled, "port": s.cfg.Port, "mailboxes": len(s.mailboxes)}
}
