// Package mailer sends email through a pluggable Transport. Mailer validates
// every Message before handing it over, and SendTemplate renders a named
// template from a template.Registry into the body first. SMTPTransport is the
// stdlib net/smtp implementation; another provider (SES, Postmark, ...) is a
// new Transport, not a change to this package.
//
// Example usage:
//
//	t, err := mailer.NewSMTPTransport(cfg) // cfg loaded from YAML
//	if err != nil { ... }
//	m := mailer.New(t)
//	err = m.Send(ctx, &mailer.Message{
//		From:    "Acme <no-reply@acme.test>",
//		To:      []string{"bia@example.com"},
//		Subject: "Welcome",
//		Text:    "Hello Bia",
//	})
package mailer

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../mocks/mailer/mock_mailer.go -package=mockmailer github.com/biairmal/go-sdk/lib/mailer Transport

import (
	"context"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/mailer/template"
)

// Transport delivers one already validated message. Implementations must be
// safe for concurrent use.
type Transport interface {
	Send(ctx context.Context, msg *Message) error
}

// Mailer validates messages and hands them to a Transport.
type Mailer struct {
	transport Transport
	templates *template.Registry
}

// Option configures a Mailer.
type Option func(*Mailer)

// WithTemplates sets the registry SendTemplate renders from. A nil registry
// is ignored.
func WithTemplates(r *template.Registry) Option {
	return func(m *Mailer) {
		if r != nil {
			m.templates = r
		}
	}
}

// New returns a Mailer sending through t.
func New(t Transport, opts ...Option) *Mailer {
	m := &Mailer{transport: t}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Send validates msg and delivers it. An invalid message returns an
// errorz.ErrBadRequest error and never reaches the Transport.
func (m *Mailer) Send(ctx context.Context, msg *Message) error {
	if err := Validate(msg); err != nil {
		return err
	}
	return m.transport.Send(ctx, msg)
}

// SendTemplate renders the named template with data into the body of a copy
// of msg, replacing its Text and HTML, then sends it like Send. msg itself is
// not modified.
func (m *Mailer) SendTemplate(ctx context.Context, msg *Message, name string, data any) error {
	if m.templates == nil {
		return errorz.Internal().WithMessage("mailer: SendTemplate needs a registry (WithTemplates)")
	}
	text, html, err := m.templates.Render(name, data)
	if err != nil {
		return err
	}
	out := *msg
	out.Text, out.HTML = text, html
	return m.Send(ctx, &out)
}
