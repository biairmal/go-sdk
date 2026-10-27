package mailer

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"strings"
	"testing"
)

// bodies decodes a parsed message into its content types and decoded bodies.
func bodies(t *testing.T, m *mail.Message) map[string]string {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("Content-Type: %v", err)
	}
	out := map[string]string{}
	if !strings.HasPrefix(mediaType, "multipart/") {
		b, err := io.ReadAll(quotedprintable.NewReader(m.Body))
		if err != nil {
			t.Fatal(err)
		}
		out[mediaType] = string(b)
		return out
	}
	out[mediaType] = ""
	mr := multipart.NewReader(m.Body, params["boundary"])
	for {
		p, err := mr.NextPart() // decodes quoted-printable itself
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatal(err)
		}
		pt, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(p)
		if err != nil {
			t.Fatal(err)
		}
		out[pt] = string(b)
	}
}

func TestEncode(t *testing.T) {
	long := strings.Repeat("Ä long line ", 20)
	tests := []struct {
		name string
		text string
		html string
		want map[string]string
	}{
		{name: "text only", text: "Hello\nBia " + long, want: map[string]string{"text/plain": "Hello\r\nBia " + long}},
		{name: "html only", html: "<p>Hi</p>", want: map[string]string{"text/html": "<p>Hi</p>"}},
		{
			name: "text and html is multipart/alternative",
			text: "Hi", html: "<p>Hi</p>",
			want: map[string]string{"multipart/alternative": "", "text/plain": "Hi", "text/html": "<p>Hi</p>"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := validMessage()
			msg.Subject = "Selamat datang, Bía"
			msg.To = []string{"bia@example.com", "Ána <ana@example.com>"}
			msg.Text, msg.HTML = tt.text, tt.html

			from, to, data, err := encode(msg)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if from != "no-reply@acme.test" || strings.Join(to, ",") != "bia@example.com,ana@example.com" {
				t.Errorf("envelope = %q -> %q, want bare addresses", from, to)
			}
			if bytes.Contains(bytes.ReplaceAll(data, []byte("\r\n"), nil), []byte("\n")) {
				t.Error("message has a bare LF; every line must end in CRLF")
			}
			for _, line := range strings.Split(string(data), "\r\n") {
				if len(line) > 998 {
					t.Errorf("line of %d bytes exceeds RFC 5322's 998", len(line))
				}
			}
			if !regexp.MustCompile(`(?m)^Subject: =\?utf-8\?q\?`).Match(data) {
				t.Errorf("subject is not Q-encoded:\n%s", data)
			}

			m, err := mail.ReadMessage(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("ReadMessage: %v", err)
			}
			var dec mime.WordDecoder
			if s, err := dec.DecodeHeader(m.Header.Get("Subject")); err != nil || s != msg.Subject {
				t.Errorf("Subject decodes to %q (%v), want %q", s, err, msg.Subject)
			}
			if got, err := m.Header.AddressList("To"); err != nil || len(got) != 2 || got[1].Name != "Ána" {
				t.Errorf("To = %v (%v), want both recipients with the display name", got, err)
			}
			for _, h := range []string{"From", "Date", "Message-ID", "MIME-Version"} {
				if m.Header.Get(h) == "" {
					t.Errorf("missing %s header", h)
				}
			}
			got := bodies(t, m)
			if len(got) != len(tt.want) {
				t.Fatalf("parts = %q, want %q", got, tt.want)
			}
			for ct, body := range tt.want {
				if got[ct] != body {
					t.Errorf("%s body = %q, want %q", ct, got[ct], body)
				}
			}
		})
	}
}

func TestEncode_RejectsInvalidMessage(t *testing.T) {
	msg := validMessage()
	msg.Subject = "x\r\nBcc: victim@example.com"
	if _, _, _, err := encode(msg); err == nil {
		t.Fatal("encode accepted a header injection")
	}
}
