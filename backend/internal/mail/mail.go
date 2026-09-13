// Package mail sends transactional email (currently: workspace invites — see
// internal/workspaces). Kept deliberately small and stdlib-only (net/smtp +
// crypto/tls) rather than pulling in a mail library, matching this
// project's "no heavy dependency for something the standard library already
// does" pattern elsewhere.
package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Message is one email to send. Callers build both bodies; HTMLBody is what
// is actually sent (see buildMIMEMessage) — TextBody exists for a future
// multipart/alternative body, not used yet.
type Message struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// Mailer sends a Message. The real implementation is SMTPMailer; tests use
// a fake so they never open a real network connection.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}

// SMTPConfig is everything needed to authenticate to and send through one
// SMTP server.
type SMTPConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	FromName    string
	// ImplicitTLS selects how the connection is secured: true for
	// implicit-TLS-from-connect ("SSL", commonly port 465), false for
	// plaintext-then-STARTTLS (commonly port 587). Getting this wrong is
	// the most common way to misconfigure SMTP — see docs/ops/email.md.
	ImplicitTLS bool
}

// SMTPMailer sends real mail over SMTP.
type SMTPMailer struct {
	cfg    SMTPConfig
	dialer *net.Dialer
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	return &SMTPMailer{cfg: cfg, dialer: &net.Dialer{Timeout: 10 * time.Second}}
}

func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)

	conn, err := m.dial(ctx, addr)
	if err != nil {
		return fmt.Errorf("dial smtp server: %w", err)
	}
	defer conn.Close() //nolint:errcheck // best-effort close, Quit() below already reports the real outcome

	client, err := smtp.NewClient(conn, m.cfg.Host)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	defer client.Close() //nolint:errcheck

	if !m.cfg.ImplicitTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: m.cfg.Host}); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}

	if m.cfg.Username != "" {
		auth := smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(m.cfg.FromAddress); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(buildMIMEMessage(m.cfg.FromName, m.cfg.FromAddress, msg)); err != nil {
		_ = w.Close()
		return fmt.Errorf("write message: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close message data: %w", err)
	}

	return client.Quit()
}

func (m *SMTPMailer) dial(ctx context.Context, addr string) (net.Conn, error) {
	if m.cfg.ImplicitTLS {
		rawConn, err := m.dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}
		tlsConn := tls.Client(rawConn, &tls.Config{ServerName: m.cfg.Host})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = rawConn.Close()
			return nil, err
		}
		return tlsConn, nil
	}
	return m.dialer.DialContext(ctx, "tcp", addr)
}

// NoopMailer logs instead of sending, for a dev/staging environment with no
// SMTP server configured. Workspace invites still "work" (the invite row and
// its link are created) — an operator can read the link from the log until
// real SMTP credentials are set.
type NoopMailer struct{}

func (NoopMailer) Send(_ context.Context, msg Message) error {
	body := msg.HTMLBody
	if body == "" {
		body = msg.TextBody
	}
	slog.Warn("SMTP not configured — logging email instead of sending it",
		"to", msg.To, "subject", msg.Subject, "body", body)
	return nil
}

func buildMIMEMessage(fromName, fromAddress string, msg Message) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s <%s>\r\n", fromName, fromAddress)
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", msg.Subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(msg.HTMLBody)
	return []byte(b.String())
}
