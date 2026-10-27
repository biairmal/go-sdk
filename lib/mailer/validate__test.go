package mailer

import (
	"errors"
	"testing"

	"github.com/biairmal/go-sdk/lib/errorz"
)

func validMessage() *Message {
	return &Message{
		From:    "Acme <no-reply@acme.test>",
		To:      []string{"bia@example.com"},
		Subject: "Welcome",
		Text:    "Hello",
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		edit    func(m *Message)
		wantErr bool
	}{
		{name: "valid text message", edit: func(*Message) {}},
		{name: "valid html only", edit: func(m *Message) { m.Text, m.HTML = "", "<p>Hi</p>" }},
		{name: "valid text and html", edit: func(m *Message) { m.HTML = "<p>Hi</p>" }},
		{name: "valid several recipients", edit: func(m *Message) { m.To = append(m.To, "Ana <ana@example.com>") }},
		{name: "valid non-ASCII subject", edit: func(m *Message) { m.Subject = "Selamat datang, Bía" }},
		{name: "missing from", edit: func(m *Message) { m.From = "" }, wantErr: true},
		{name: "malformed from", edit: func(m *Message) { m.From = "not-an-address" }, wantErr: true},
		{name: "two addresses in from", edit: func(m *Message) { m.From = "a@acme.test, b@acme.test" }, wantErr: true},
		{name: "no recipients", edit: func(m *Message) { m.To = nil }, wantErr: true},
		{name: "empty recipient", edit: func(m *Message) { m.To = []string{" "} }, wantErr: true},
		{name: "malformed recipient", edit: func(m *Message) { m.To = []string{"bia@"} }, wantErr: true},
		{name: "missing subject", edit: func(m *Message) { m.Subject = "  " }, wantErr: true},
		{name: "no body", edit: func(m *Message) { m.Text = "" }, wantErr: true},
		{
			name:    "header injection in subject",
			edit:    func(m *Message) { m.Subject = "Hi\r\nBcc: victim@example.com" },
			wantErr: true,
		},
		{name: "bare LF in subject", edit: func(m *Message) { m.Subject = "Hi\nBcc: x@example.com" }, wantErr: true},
		{
			name:    "header injection in from",
			edit:    func(m *Message) { m.From = "a@acme.test\r\nBcc: victim@example.com" },
			wantErr: true,
		},
		{
			name:    "header injection in to",
			edit:    func(m *Message) { m.To = []string{"bia@example.com\nBcc: victim@example.com"} },
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validMessage()
			tt.edit(m)
			err := Validate(m)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, errorz.ErrBadRequest) {
				t.Errorf("Validate() error = %v, want errorz.ErrBadRequest", err)
			}
		})
	}
}

func TestValidate_NilMessage(t *testing.T) {
	if err := Validate(nil); !errors.Is(err, errorz.ErrBadRequest) {
		t.Fatalf("Validate(nil) = %v, want errorz.ErrBadRequest", err)
	}
}
