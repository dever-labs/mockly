// Package smtpserver implements an SMTP mock server that captures inbound emails.
package smtpserver

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/mail"
	"regexp"
	"strings"
	"sync"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/rs/xid"

	"github.com/dever-labs/mockly/internal/config"
	"github.com/dever-labs/mockly/internal/logger"
)

// Server is the SMTP mock server.
type Server struct {
	cfg   *config.SMTPConfig
	log   *logger.Logger
	rules []config.SMTPRule
	inbox *Inbox
}

// Inbox is a bounded, thread-safe store of received emails.
type Inbox struct {
	mu       sync.RWMutex
	emails   []config.ReceivedEmail
	maxSize  int
}

func newInbox(maxSize int) *Inbox {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &Inbox{maxSize: maxSize}
}

// NewInbox creates a new Inbox with the given capacity. Exported for testing.
func NewInbox(maxSize int) *Inbox { return newInbox(maxSize) }

func (b *Inbox) Add(e config.ReceivedEmail) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.emails) >= b.maxSize {
		b.emails = b.emails[1:]
	}
	b.emails = append(b.emails, e)
}

func (b *Inbox) All() []config.ReceivedEmail {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]config.ReceivedEmail, len(b.emails))
	copy(out, b.emails)
	return out
}

func (b *Inbox) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.emails = nil
}

// New creates a Server.
func New(cfg *config.SMTPConfig, log *logger.Logger) *Server {
	return &Server{
		cfg:   cfg,
		log:   log,
		rules: append([]config.SMTPRule(nil), cfg.Rules...),
		inbox: newInbox(cfg.MaxEmails),
	}
}

func (s *Server) SetRules(rules []config.SMTPRule) {
	s.rules = append([]config.SMTPRule(nil), rules...)
}

func (s *Server) GetRules() []config.SMTPRule {
	return append([]config.SMTPRule(nil), s.rules...)
}

func (s *Server) GetInbox() *Inbox {
	return s.inbox
}

// Start begins listening. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	srv := gosmtp.NewServer(&backend{server: s})
	srv.Addr = fmt.Sprintf(":%d", s.cfg.Port)
	srv.Domain = s.cfg.Domain
	srv.AllowInsecureAuth = true
	srv.ReadTimeout = 30 * time.Second
	srv.WriteTimeout = 30 * time.Second

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case <-ctx.Done():
		return srv.Close()
	case err := <-errCh:
		return err
	}
}

// matchRule returns the action and message for the first matching rule.
// Returns "accept", "" if no rule matches.
func (s *Server) matchRule(from, to, subject string) (action, message string) {
	for _, rule := range s.rules {
		if rule.From != "" && !matchSMTPPattern(rule.From, from) {
			continue
		}
		if rule.To != "" && !matchSMTPPattern(rule.To, to) {
			continue
		}
		if rule.Subject != "" && !matchSMTPPattern(rule.Subject, subject) {
			continue
		}
		return rule.Action, rule.Message
	}
	return "accept", ""
}

func matchSMTPPattern(pattern, value string) bool {
	if strings.HasPrefix(pattern, "re:") {
		re, err := regexp.Compile(pattern[3:])
		if err != nil {
			return false
		}
		return re.MatchString(value)
	}
	if strings.Contains(pattern, "*") {
		parts := strings.SplitN(pattern, "*", 2)
		return strings.HasPrefix(value, parts[0]) &&
			(parts[1] == "" || strings.HasSuffix(value, parts[1]))
	}
	return strings.EqualFold(pattern, value)
}

// StatusInfo returns JSON-serialisable server info.
func (s *Server) StatusInfo() map[string]interface{} {
	return map[string]interface{}{
		"protocol": "smtp",
		"enabled":  s.cfg.Enabled,
		"port":     s.cfg.Port,
		"domain":   s.cfg.Domain,
		"emails":   len(s.inbox.All()),
		"rules":    len(s.rules),
	}
}

// ---------------------------------------------------------------------------
// go-smtp Backend + Session implementation
// ---------------------------------------------------------------------------

type backend struct {
	server *Server
}

func (b *backend) NewSession(c *gosmtp.Conn) (gosmtp.Session, error) {
	return &session{server: b.server}, nil
}

type session struct {
	server  *Server
	from    string
	to      []string
}

func (s *session) AuthPlain(username, password string) error {
	return nil // accept any credentials
}

func (s *session) Mail(from string, opts *gosmtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *session) Rcpt(to string, opts *gosmtp.RcptOptions) error {
	s.to = append(s.to, to)
	return nil
}

func (s *session) Data(r io.Reader) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	// Parse subject from the message headers.
	subject := ""
	if msg, err := mail.ReadMessage(bytes.NewReader(raw)); err == nil {
		subject = msg.Header.Get("Subject")
	}

	// Check rules against each recipient.
	for _, to := range s.to {
		action, message := s.server.matchRule(s.from, to, subject)
		if action == "reject" {
			errMsg := message
			if errMsg == "" {
				errMsg = "Message rejected by policy"
			}
			return &gosmtp.SMTPError{
				Code:         550,
				EnhancedCode: gosmtp.EnhancedCode{5, 7, 1},
				Message:      errMsg,
			}
		}
	}

	email := config.ReceivedEmail{
		ID:         xid.New().String(),
		From:       s.from,
		To:         append([]string(nil), s.to...),
		Subject:    subject,
		Body:       string(raw),
		ReceivedAt: time.Now().UTC().Format(time.RFC3339),
	}
	s.server.inbox.Add(email)

	s.server.log.Log(logger.Entry{
		Protocol: "smtp",
		Method:   "DATA",
		Path:     fmt.Sprintf("%s → %s", s.from, strings.Join(s.to, ", ")),
		Status:   250,
		Body:     subject,
	})
	return nil
}

func (s *session) Reset() {
	s.from = ""
	s.to = nil
}

func (s *session) Logout() error {
	return nil
}
