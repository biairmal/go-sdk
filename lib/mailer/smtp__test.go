package mailer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// session is what the test SMTP server saw in one connection.
type session struct {
	tls  bool
	auth string
	from string
	to   []string
	data string
}

// testSMTP is a minimal SMTP server on a local listener: EHLO, STARTTLS,
// AUTH, MAIL, RCPT, DATA and QUIT, enough to drive net/smtp end to end.
type testSMTP struct {
	port          int
	tlsConfig     *tls.Config
	offerStartTLS bool
	sessions      chan session
}

// testCerts borrows httptest's certificate for 127.0.0.1: the server side
// config and a client config that trusts it.
func testCerts(t *testing.T) (server, client *tls.Config) {
	t.Helper()
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	defer srv.Close()
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return &tls.Config{Certificates: srv.TLS.Certificates, MinVersion: tls.VersionTLS12},
		&tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
}

func startSMTP(t *testing.T, serverTLS *tls.Config, implicit, offerStartTLS bool) *testSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &testSMTP{
		port:          ln.Addr().(*net.TCPAddr).Port,
		tlsConfig:     serverTLS,
		offerStartTLS: offerStartTLS,
		sessions:      make(chan session, 1),
	}
	if implicit {
		ln = tls.NewListener(ln, serverTLS)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(conn)
		}
	}()
	return s
}

func (s *testSMTP) serve(conn net.Conn) {
	defer func() { conn.Close() }()
	tp := textproto.NewConn(conn)
	_, isTLS := conn.(*tls.Conn)
	got := session{tls: isTLS}
	tp.PrintfLine("220 test ESMTP")
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		verb, arg, _ := strings.Cut(line, " ")
		switch strings.ToUpper(verb) {
		case "EHLO":
			tp.PrintfLine("250-test")
			if s.offerStartTLS && !got.tls {
				tp.PrintfLine("250-STARTTLS")
			}
			tp.PrintfLine("250 AUTH PLAIN")
		case "STARTTLS":
			if !s.offerStartTLS || got.tls {
				tp.PrintfLine("502 not supported")
				continue
			}
			tp.PrintfLine("220 ready")
			tc := tls.Server(conn, s.tlsConfig)
			if tc.Handshake() != nil {
				return
			}
			conn, tp, got.tls = tc, textproto.NewConn(tc), true
		case "AUTH":
			got.auth = arg
			tp.PrintfLine("235 ok")
		case "MAIL":
			got.from = arg
			tp.PrintfLine("250 ok")
		case "RCPT":
			got.to = append(got.to, arg)
			tp.PrintfLine("250 ok")
		case "DATA":
			tp.PrintfLine("354 go on")
			b, err := tp.ReadDotBytes()
			if err != nil {
				return
			}
			got.data = string(b)
			tp.PrintfLine("250 queued")
		case "QUIT":
			tp.PrintfLine("221 bye")
			s.sessions <- got
			return
		default:
			tp.PrintfLine("502 unknown")
		}
	}
}

func TestSMTPTransport_Send(t *testing.T) {
	serverTLS, clientTLS := testCerts(t)
	plainAuth := "PLAIN " + base64.StdEncoding.EncodeToString([]byte("\x00user\x00secret"))
	tests := []struct {
		name          string
		mode          string
		user          string
		implicit      bool
		offerStartTLS bool
		wantTLS       bool
		wantErr       bool
	}{
		{name: "plain text, no auth", mode: TLSNone},
		{name: "plain text with auth on localhost", mode: TLSNone, user: "user"},
		{name: "starttls with auth", mode: TLSStartTLS, user: "user", offerStartTLS: true, wantTLS: true},
		{name: "implicit tls with auth", mode: TLSImplicit, user: "user", implicit: true, wantTLS: true},
		{name: "starttls required but not offered", mode: TLSStartTLS, user: "user", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := startSMTP(t, serverTLS, tt.implicit, tt.offerStartTLS)
			tr, err := NewSMTPTransport(&SMTPConfig{
				Host: "127.0.0.1", Port: srv.port, TLS: tt.mode, Username: tt.user, Password: "secret",
			}, WithTLSConfig(clientTLS))
			if err != nil {
				t.Fatalf("NewSMTPTransport: %v", err)
			}
			msg := validMessage()
			msg.To = append(msg.To, "Ana <ana@example.com>")

			err = tr.Send(context.Background(), msg)
			if tt.wantErr {
				var ez *errorz.Error
				if !errors.As(err, &ez) || ez.Code != errorz.CodeBadGateway {
					t.Fatalf("Send() = %v, want an errorz.CodeBadGateway error", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Send: %v", err)
			}
			var got session
			select {
			case got = <-srv.sessions:
			case <-time.After(5 * time.Second):
				t.Fatal("the server never saw QUIT")
			}
			if got.tls != tt.wantTLS {
				t.Errorf("session TLS = %v, want %v", got.tls, tt.wantTLS)
			}
			if wantAuth := map[bool]string{true: plainAuth}[tt.user != ""]; got.auth != wantAuth {
				t.Errorf("AUTH = %q, want %q", got.auth, wantAuth)
			}
			if got.from != "FROM:<no-reply@acme.test>" {
				t.Errorf("MAIL %s", got.from)
			}
			if strings.Join(got.to, " ") != "TO:<bia@example.com> TO:<ana@example.com>" {
				t.Errorf("RCPT %q", got.to)
			}
			if !strings.Contains(got.data, "Subject: Welcome\n") || !strings.HasSuffix(got.data, "Hello\n") {
				t.Errorf("DATA =\n%s", got.data)
			}
		})
	}
}

func TestSMTPTransport_SendInvalidMessageDoesNotDial(t *testing.T) {
	tr, err := NewSMTPTransport(&SMTPConfig{Host: "127.0.0.1", Port: 1, TLS: TLSNone})
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Send(context.Background(), &Message{}); !errors.Is(err, errorz.ErrBadRequest) {
		t.Fatalf("Send(invalid) = %v, want errorz.ErrBadRequest", err)
	}
}

func TestSMTPTransport_TimeoutOnSilentServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept() // accept, then never greet
		if err == nil {
			defer conn.Close()
			time.Sleep(5 * time.Second)
		}
	}()
	tr, err := NewSMTPTransport(&SMTPConfig{
		Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, TLS: TLSNone, Timeout: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := tr.Send(context.Background(), validMessage()); err == nil {
		t.Fatal("Send succeeded against a silent server")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("Send took %v; the timeout did not bound it", d)
	}
}

func TestNewSMTPTransport_Config(t *testing.T) {
	tests := []struct {
		name     string
		cfg      SMTPConfig
		wantPort int
		wantErr  bool
	}{
		{name: "defaults to starttls on 587", cfg: SMTPConfig{Host: "smtp.test"}, wantPort: 587},
		{name: "implicit defaults to 465", cfg: SMTPConfig{Host: "smtp.test", TLS: TLSImplicit}, wantPort: 465},
		{name: "explicit port kept", cfg: SMTPConfig{Host: "smtp.test", Port: 2525, TLS: TLSNone}, wantPort: 2525},
		{name: "missing host", cfg: SMTPConfig{}, wantErr: true},
		{name: "port out of range", cfg: SMTPConfig{Host: "smtp.test", Port: 70000}, wantErr: true},
		{name: "unknown tls mode", cfg: SMTPConfig{Host: "smtp.test", TLS: "ssl"}, wantErr: true},
		{name: "negative timeout", cfg: SMTPConfig{Host: "smtp.test", Timeout: -time.Second}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, err := NewSMTPTransport(&tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewSMTPTransport() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && (tr.cfg.Port != tt.wantPort || tr.tlsConfig.ServerName != "smtp.test") {
				t.Errorf("port = %d, server name = %q", tr.cfg.Port, tr.tlsConfig.ServerName)
			}
		})
	}
}
