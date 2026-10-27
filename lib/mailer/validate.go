package mailer

import (
	"net/mail"
	"strings"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// Validate reports whether msg can be sent: From, at least one To and Subject
// are present, every address parses (net/mail.ParseAddress, one address per
// field), no header field contains a CR or LF (header injection), and there is
// a Text or HTML body. The error wraps errorz.ErrBadRequest. Messages never
// echo the addresses, so the error is safe to log.
func Validate(msg *Message) error {
	_, _, err := parse(msg)
	return err
}

// parse validates msg and returns its parsed sender and recipients.
func parse(msg *Message) (from *mail.Address, to []*mail.Address, err error) {
	if msg == nil {
		return nil, nil, invalid("message is nil")
	}
	if strings.TrimSpace(msg.Subject) == "" {
		return nil, nil, invalid("subject is required")
	}
	if strings.ContainsAny(msg.Subject, "\r\n") {
		return nil, nil, invalid("subject contains a line break")
	}
	if msg.Text == "" && msg.HTML == "" {
		return nil, nil, invalid("a text or HTML body is required")
	}
	if from, err = address("from", msg.From); err != nil {
		return nil, nil, err
	}
	if len(msg.To) == 0 {
		return nil, nil, invalid("at least one to address is required")
	}
	to = make([]*mail.Address, len(msg.To))
	for i, s := range msg.To {
		if to[i], err = address("to", s); err != nil {
			return nil, nil, err
		}
	}
	return from, to, nil
}

// address parses one header address, rejecting line breaks before parsing.
func address(field, s string) (*mail.Address, error) {
	if strings.TrimSpace(s) == "" {
		return nil, invalid(field + " address is required")
	}
	if strings.ContainsAny(s, "\r\n") {
		return nil, invalid(field + " address contains a line break")
	}
	a, err := mail.ParseAddress(s)
	if err != nil {
		return nil, invalid("invalid " + field + " address")
	}
	return a, nil
}

func invalid(reason string) error {
	return errorz.BadRequest().WithMessage("mailer: " + reason)
}
