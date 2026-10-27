package mailer

import (
	"context"
	"crypto/tls"
	"net"
	"net/smtp"
	"strconv"
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// TLS modes for SMTPConfig.TLS.
const (
	// TLSStartTLS connects in plain text and upgrades with STARTTLS before
	// authenticating. The send fails if the server does not offer it.
	// Usually port 587.
	TLSStartTLS = "starttls"
	// TLSImplicit speaks TLS from the first byte. Usually port 465.
	TLSImplicit = "implicit"
	// TLSNone never encrypts. For a local relay or a test server only;
	// net/smtp then refuses to send credentials to anything but localhost.
	TLSNone = "none"
)

// SMTPConfig holds the YAML-able SMTP settings.
type SMTPConfig struct {
	// Host is the SMTP server name, also used to verify its certificate.
	Host string `mapstructure:"host"`
	// Port defaults to 465 for TLSImplicit and 587 otherwise.
	Port int `mapstructure:"port"`
	// Username and Password enable AUTH PLAIN when Username is set.
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	// TLS is one of TLSStartTLS (default), TLSImplicit or TLSNone.
	TLS string `mapstructure:"tls"`
	// Timeout bounds one whole send: dial, handshake and transfer.
	Timeout time.Duration `mapstructure:"timeout"`
}

// DefaultSMTPConfig returns STARTTLS on port 587 with a 30s timeout. Host is
// left empty: it has no sane default.
func DefaultSMTPConfig() SMTPConfig {
	return SMTPConfig{Port: 587, TLS: TLSStartTLS, Timeout: 30 * time.Second}
}

// Validate checks that c is well-formed.
func (c *SMTPConfig) Validate() error {
	if c.Host == "" {
		return errorz.BadRequest().WithMessage("mailer: smtp host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return errorz.BadRequest().WithMessage("mailer: smtp port must be in 1..65535")
	}
	if c.TLS != TLSStartTLS && c.TLS != TLSImplicit && c.TLS != TLSNone {
		return errorz.BadRequest().WithMessage("mailer: smtp tls must be starttls, implicit or none")
	}
	if c.Timeout <= 0 {
		return errorz.BadRequest().WithMessage("mailer: smtp timeout must be positive")
	}
	return nil
}

// SMTPTransport is a Transport over stdlib net/smtp. Each Send opens its own
// connection, so it is safe for concurrent use.
type SMTPTransport struct {
	cfg       SMTPConfig
	tlsConfig *tls.Config
}

// SMTPOption configures an SMTPTransport.
type SMTPOption func(*SMTPTransport)

// WithTLSConfig sets the TLS client config, e.g. to trust a private CA.
// ServerName defaults to SMTPConfig.Host. A nil config is ignored.
func WithTLSConfig(c *tls.Config) SMTPOption {
	return func(t *SMTPTransport) {
		if c != nil {
			t.tlsConfig = c.Clone()
		}
	}
}

// NewSMTPTransport fills zero-valued fields of cfg from DefaultSMTPConfig and
// validates the result.
func NewSMTPTransport(cfg *SMTPConfig, opts ...SMTPOption) (*SMTPTransport, error) {
	c := *cfg
	def := DefaultSMTPConfig()
	if c.TLS == "" {
		c.TLS = def.TLS
	}
	if c.Port == 0 {
		c.Port = def.Port
		if c.TLS == TLSImplicit {
			c.Port = 465
		}
	}
	if c.Timeout == 0 {
		c.Timeout = def.Timeout
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	t := &SMTPTransport{cfg: c, tlsConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	for _, opt := range opts {
		opt(t)
	}
	if t.tlsConfig.ServerName == "" {
		t.tlsConfig.ServerName = c.Host
	}
	return t, nil
}

// Send validates and delivers msg in one SMTP session. A delivery failure
// returns an errorz.CodeBadGateway error wrapping the cause (a
// *textproto.Error carries the server's reply code).
func (t *SMTPTransport) Send(ctx context.Context, msg *Message) error {
	from, to, data, err := encode(msg)
	if err != nil {
		return err
	}
	if err := t.send(ctx, from, to, data); err != nil {
		return errorz.Wrap(err).WithCode(errorz.CodeBadGateway).WithMessage("mailer: smtp send failed")
	}
	return nil
}

func (t *SMTPTransport) send(ctx context.Context, from string, to []string, data []byte) error {
	ctx, cancel := context.WithTimeout(ctx, t.cfg.Timeout)
	defer cancel()

	addr := net.JoinHostPort(t.cfg.Host, strconv.Itoa(t.cfg.Port))
	var conn net.Conn
	var err error
	if t.cfg.TLS == TLSImplicit {
		conn, err = (&tls.Dialer{Config: t.tlsConfig}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	// net/smtp takes no context: a deadline bounds a slow server and closing
	// the connection aborts the session when ctx is canceled.
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return err
		}
	}
	defer context.AfterFunc(ctx, func() { _ = conn.Close() })()
	return t.session(conn, from, to, data)
}

// session runs one SMTP conversation on conn: optional STARTTLS and AUTH,
// then the envelope, the data and QUIT.
func (t *SMTPTransport) session(conn net.Conn, from string, to []string, data []byte) error {
	c, err := smtp.NewClient(conn, t.cfg.Host)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()
	if t.cfg.TLS == TLSStartTLS {
		if err := c.StartTLS(t.tlsConfig); err != nil {
			return err
		}
	}
	if t.cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", t.cfg.Username, t.cfg.Password, t.cfg.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
