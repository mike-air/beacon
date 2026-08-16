// Package email sends transactional mail behind a small Sender interface. It is
// never called inline from a request handler — the queue boundary (Chapter 23)
// means a background job calls Send, so a slow or down SMTP server never slows a
// request.
//
// Course mapping: Chapter 23 — transactional email. Two implementations: an
// SMTP sender built on github.com/wneessen/go-mail (pointed at Mailpit in dev),
// and a LogSender used when SMTP is unconfigured so `make run` works with only
// Postgres.
//
// NOTE (deviation): the course wires the From address into the message inside
// the worker; we keep From on the sender (set once at construction) so callers
// only pass to/subject/body.
package email

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/wneessen/go-mail"
)

// Sender delivers one message. htmlBody and textBody are both optional, but at
// least one should be non-empty.
type Sender interface {
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}

// LogSender writes the message to the logger instead of sending it. This is the
// default when no SMTP host is configured: the service boots and "email" works
// (you read it in the logs) with nothing but Postgres running.
type LogSender struct {
	from   string
	logger *slog.Logger
}

// NewLogSender returns a Sender that logs messages.
func NewLogSender(from string, logger *slog.Logger) *LogSender {
	return &LogSender{from: from, logger: logger}
}

// Send logs the message at info level.
func (l *LogSender) Send(_ context.Context, to, subject, htmlBody, textBody string) error {
	l.logger.Info("email (log sender)",
		slog.String("from", l.from),
		slog.String("to", to),
		slog.String("subject", subject),
		slog.Int("html_bytes", len(htmlBody)),
		slog.Int("text_bytes", len(textBody)),
	)
	return nil
}

// SMTPConfig carries the SMTP_* settings.
type SMTPConfig struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

// SMTPSender sends real mail over SMTP via go-mail. In dev it points at Mailpit
// (host localhost, port 1025, no auth).
type SMTPSender struct {
	client *mail.Client
	from   string
}

// NewSMTPSender builds an SMTP sender. With empty user/pass it uses no auth
// (Mailpit's mode); otherwise it uses LOGIN auth.
func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	opts := []mail.Option{
		mail.WithPort(cfg.Port),
		// Mailpit speaks plain SMTP with no TLS and no auth, so default to that
		// and only switch on auth/TLS when credentials are supplied.
		mail.WithTLSPolicy(mail.NoTLS),
	}
	if cfg.User != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthLogin),
			mail.WithUsername(cfg.User),
			mail.WithPassword(cfg.Pass),
			mail.WithTLSPolicy(mail.TLSOpportunistic),
		)
	}
	client, err := mail.NewClient(cfg.Host, opts...)
	if err != nil {
		return nil, fmt.Errorf("email.NewSMTPSender: %w", err)
	}
	return &SMTPSender{client: client, from: cfg.From}, nil
}

// Send composes a multipart message and dials the SMTP server to deliver it.
func (s *SMTPSender) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	msg := mail.NewMsg()
	if err := msg.From(s.from); err != nil {
		return fmt.Errorf("email.Send: from: %w", err)
	}
	if err := msg.To(to); err != nil {
		return fmt.Errorf("email.Send: to: %w", err)
	}
	msg.Subject(subject)
	if textBody != "" {
		msg.SetBodyString(mail.TypeTextPlain, textBody)
	}
	if htmlBody != "" {
		if textBody != "" {
			msg.AddAlternativeString(mail.TypeTextHTML, htmlBody)
		} else {
			msg.SetBodyString(mail.TypeTextHTML, htmlBody)
		}
	}
	if err := s.client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("email.Send: deliver: %w", err)
	}
	return nil
}

// New picks the right Sender for the config: a real SMTPSender when a host is
// set, otherwise the LogSender. It never returns a nil Sender, so callers can
// always send (the message just lands in the logs in dev).
func New(cfg SMTPConfig, logger *slog.Logger) (Sender, error) {
	if cfg.Host == "" {
		return NewLogSender(cfg.From, logger), nil
	}
	return NewSMTPSender(cfg)
}
