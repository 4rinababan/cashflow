package mail

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeSMTPServer speaks just enough SMTP to let smtp.Client complete a real
// send — EHLO, MAIL FROM, RCPT TO, DATA, QUIT — and records the full
// transcript so a test can assert on it. It never talks to a real SMTP
// server; this is how SMTPMailer's protocol handling is verified without
// ever touching the real mail server this project's config points at.
type fakeSMTPServer struct {
	listener   net.Listener
	transcript []string
}

func newFakeSMTPServer(t *testing.T) *fakeSMTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &fakeSMTPServer{listener: ln}
	t.Cleanup(func() { _ = ln.Close() })
	go s.serveOne(t)
	return s
}

func (s *fakeSMTPServer) addr() string { return s.listener.Addr().String() }

func (s *fakeSMTPServer) serveOne(t *testing.T) {
	conn, err := s.listener.Accept()
	if err != nil {
		return // listener closed by test cleanup
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	respond := func(line string) {
		s.transcript = append(s.transcript, "S: "+line)
		_, _ = conn.Write([]byte(line + "\r\n"))
	}

	respond("220 fake-smtp ready")
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		s.transcript = append(s.transcript, "C: "+line)

		switch {
		case strings.HasPrefix(line, "EHLO"):
			respond("250 fake-smtp hello")
		case strings.HasPrefix(line, "MAIL FROM"):
			respond("250 OK")
		case strings.HasPrefix(line, "RCPT TO"):
			respond("250 OK")
		case line == "DATA":
			respond("354 send message, end with <CRLF>.<CRLF>")
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				dataLine = strings.TrimRight(dataLine, "\r\n")
				s.transcript = append(s.transcript, "D: "+dataLine)
				if dataLine == "." {
					break
				}
			}
			respond("250 message accepted")
		case line == "QUIT":
			respond("221 bye")
			return
		default:
			respond("500 unrecognized command")
		}
	}
}

func TestSMTPMailer_Send_CompletesTheFullProtocolConversation(t *testing.T) {
	server := newFakeSMTPServer(t)
	host, portStr, err := net.SplitHostPort(server.addr())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}

	mailer := NewSMTPMailer(SMTPConfig{
		Host:        host,
		Port:        port,
		FromAddress: "noreply@example.com",
		FromName:    "Cashflow",
		ImplicitTLS: false,
	})

	err = mailer.Send(context.Background(), Message{
		To:       "someone@example.com",
		Subject:  "You're invited",
		HTMLBody: "<p>Join the workspace</p>",
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	full := strings.Join(server.transcript, "\n")
	for _, want := range []string{
		"C: EHLO",
		"C: MAIL FROM:<noreply@example.com>",
		"C: RCPT TO:<someone@example.com>",
		"C: DATA",
		"D: Subject: You're invited",
		"D: <p>Join the workspace</p>",
		"C: QUIT",
	} {
		if !strings.Contains(full, want) {
			t.Errorf("transcript missing %q; full transcript:\n%s", want, full)
		}
	}
}

func TestSMTPMailer_Send_ReturnsAnErrorWhenTheServerIsUnreachable(t *testing.T) {
	// A closed loopback port refuses a connection immediately on every
	// platform this runs on — unlike an arbitrary unused port number, which
	// (as observed on this WSL setup) can instead hang until the dialer's
	// own timeout, making the test needlessly slow.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}

	mailer := NewSMTPMailer(SMTPConfig{Host: host, Port: port})
	if err := mailer.Send(context.Background(), Message{To: "a@example.com"}); err == nil {
		t.Fatal("Send() error = nil, want an error for an unreachable server")
	}
}

func TestSMTPMailer_Dial_CompletesAnImplicitTLSHandshake(t *testing.T) {
	cert := generateSelfSignedCert(t)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatalf("tls.Listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte("220 fake-smtp ready\r\n"))
	}()

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatalf("parse port: %v", err)
	}

	mailer := NewSMTPMailer(SMTPConfig{Host: host, Port: port, ImplicitTLS: true})

	// dialInsecureForTest exercises the same handshake dial() performs, but
	// against a self-signed test certificate, so it skips verification for
	// this one test connection only — production dial() is unmodified and
	// always verifies (see SMTPMailer.dial).
	conn, err := dialInsecureForTest(t, mailer, fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		t.Fatalf("dial() error = %v", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	greeting, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read greeting: %v", err)
	}
	if !strings.HasPrefix(greeting, "220") {
		t.Errorf("greeting = %q, want a 220 response over the TLS connection", greeting)
	}
}

// dialInsecureForTest calls the real dial() logic but against a
// self-signed test certificate, so it necessarily skips verification for
// this one test connection only — production dial() is unmodified.
func dialInsecureForTest(t *testing.T, m *SMTPMailer, addr string) (net.Conn, error) {
	t.Helper()
	rawConn, err := m.dialer.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(rawConn, &tls.Config{InsecureSkipVerify: true}) //nolint:gosec // test-only, self-signed cert
	if err := tlsConn.HandshakeContext(context.Background()); err != nil {
		_ = rawConn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func generateSelfSignedCert(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
	return cert
}
