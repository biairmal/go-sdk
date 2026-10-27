package mailer

import (
	"bytes"
	"crypto/rand"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/textproto"
	"strings"
	"time"
)

// Message is one email. Addresses use net/mail syntax and may carry a
// display name ("Acme <no-reply@acme.test>"). Set Text, HTML or both: with
// both, the mail is multipart/alternative and the client picks one.
type Message struct {
	From    string
	To      []string
	Subject string
	Text    string
	HTML    string
}

// encode validates msg and returns the SMTP envelope (bare sender and
// recipient addresses) and the RFC 5322 message: CRLF line endings, a
// Q-encoded subject, quoted-printable UTF-8 bodies.
func encode(msg *Message) (from string, to []string, data []byte, err error) {
	sender, rcpts, err := parse(msg)
	if err != nil {
		return "", nil, nil, err
	}
	to = make([]string, len(rcpts))
	toHeader := make([]string, len(rcpts))
	for i, a := range rcpts {
		to[i], toHeader[i] = a.Address, a.String()
	}
	domain := sender.Address[strings.LastIndex(sender.Address, "@")+1:]

	var b bytes.Buffer
	header := func(k, v string) { b.WriteString(k + ": " + v + "\r\n") }
	header("From", sender.String())
	header("To", strings.Join(toHeader, ", "))
	header("Subject", mime.QEncoding.Encode("utf-8", msg.Subject))
	header("Date", time.Now().Format(time.RFC1123Z))
	header("Message-ID", "<"+rand.Text()+"@"+domain+">")
	header("MIME-Version", "1.0")

	if msg.Text != "" && msg.HTML != "" {
		mw := multipart.NewWriter(&b)
		header("Content-Type", mime.FormatMediaType("multipart/alternative", map[string]string{"boundary": mw.Boundary()}))
		b.WriteString("\r\n")
		if err = writePart(mw, "text/plain", msg.Text); err == nil {
			if err = writePart(mw, "text/html", msg.HTML); err == nil {
				err = mw.Close()
			}
		}
	} else {
		ct, body := "text/plain", msg.Text
		if msg.HTML != "" {
			ct, body = "text/html", msg.HTML
		}
		header("Content-Type", ct+"; charset=utf-8")
		header("Content-Transfer-Encoding", "quoted-printable")
		b.WriteString("\r\n")
		err = writeQP(&b, body)
	}
	if err != nil {
		return "", nil, nil, err
	}
	return sender.Address, to, b.Bytes(), nil
}

// writePart adds one quoted-printable UTF-8 part to a multipart body.
func writePart(mw *multipart.Writer, contentType, body string) error {
	w, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {contentType + "; charset=utf-8"},
		"Content-Transfer-Encoding": {"quoted-printable"},
	})
	if err != nil {
		return err
	}
	return writeQP(w, body)
}

// writeQP writes body quoted-printable encoded; line breaks become CRLF.
func writeQP(w io.Writer, body string) error {
	qp := quotedprintable.NewWriter(w)
	if _, err := qp.Write([]byte(body)); err != nil {
		return err
	}
	return qp.Close()
}
