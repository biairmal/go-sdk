package mailer

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/mailer/template"
)

// fakeTransport records what it is asked to deliver: Mailer needs no SMTP.
type fakeTransport struct {
	sent []Message
	err  error
}

func (f *fakeTransport) Send(_ context.Context, msg *Message) error {
	f.sent = append(f.sent, *msg)
	return f.err
}

func TestMailer_Send(t *testing.T) {
	failure := errors.New("transport down")
	tests := []struct {
		name     string
		msg      *Message
		tErr     error
		wantSent int
		wantErr  error
	}{
		{name: "valid message is delivered", msg: validMessage(), wantSent: 1},
		{name: "invalid message never reaches the transport", msg: &Message{}, wantErr: errorz.ErrBadRequest},
		{name: "transport error is returned", msg: validMessage(), tErr: failure, wantSent: 1, wantErr: failure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ft := &fakeTransport{err: tt.tErr}
			err := New(ft).Send(context.Background(), tt.msg)
			if !errors.Is(err, tt.wantErr) || (tt.wantErr == nil && err != nil) {
				t.Fatalf("Send() = %v, want %v", err, tt.wantErr)
			}
			if len(ft.sent) != tt.wantSent {
				t.Errorf("transport got %d messages, want %d", len(ft.sent), tt.wantSent)
			}
		})
	}
}

func TestMailer_SendTemplate(t *testing.T) {
	reg, err := template.New(fstest.MapFS{
		"welcome.txt":  {Data: []byte("Hello {{.Name}}")},
		"welcome.html": {Data: []byte("<p>Hello {{.Name}}</p>")},
	})
	if err != nil {
		t.Fatal(err)
	}
	ft := &fakeTransport{}
	m := New(ft, WithTemplates(reg))
	msg := validMessage()
	msg.Text = ""

	if err := m.SendTemplate(context.Background(), msg, "welcome", map[string]string{"Name": "<Bia>"}); err != nil {
		t.Fatalf("SendTemplate: %v", err)
	}
	if len(ft.sent) != 1 {
		t.Fatalf("transport got %d messages, want 1", len(ft.sent))
	}
	got := ft.sent[0]
	if got.Text != "Hello <Bia>" || got.HTML != "<p>Hello &lt;Bia&gt;</p>" {
		t.Errorf("rendered Text = %q, HTML = %q", got.Text, got.HTML)
	}
	if msg.Text != "" || msg.HTML != "" {
		t.Error("SendTemplate modified the caller's message")
	}

	if err := m.SendTemplate(context.Background(), msg, "missing", nil); !errors.Is(err, errorz.ErrNotFound) {
		t.Errorf("unknown template = %v, want errorz.ErrNotFound", err)
	}
	if err := New(ft).SendTemplate(context.Background(), msg, "welcome", nil); err == nil {
		t.Error("SendTemplate without a registry succeeded")
	}
}
