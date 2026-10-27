# Mailer Package

`mailer` sends email through a pluggable `Transport`: it validates every message, builds a proper MIME message, and ships an SMTP transport over the standard library. The `mailer/template` subpackage renders named `text/template` and `html/template` bodies.

## Overview

A service that sends mail needs the same few things every time: reject a malformed or injected message before it leaves, encode it correctly (CRLF, UTF-8 subject, text and HTML alternatives), and talk to an SMTP server over TLS. `Mailer` does the first and hands the message to a `Transport`; `SMTPTransport` does the rest. Another provider (SES, Postmark, ...) is a new `Transport` implementation, not a change to this package.

## Features

- **`Validate`**: From, at least one To and Subject are required; every address must parse with `net/mail.ParseAddress` (one address per entry, display names allowed); a CR or LF in any header field is rejected (header injection); a Text or HTML body is required. Errors wrap `errorz.ErrBadRequest` and never echo the addresses.
- **`Mailer`**: validates, then delivers through its `Transport`. `SendTemplate` renders a named template into a copy of the message (the caller's message is not modified).
- **MIME**: CRLF line endings, `Subject` Q-encoded (`mime.QEncoding`) when non-ASCII, quoted-printable UTF-8 bodies, `multipart/alternative` when both Text and HTML are set, `Date` and `Message-ID` headers.
- **`SMTPTransport`**: stdlib `net/smtp`; `starttls` (default, fails if the server does not offer it), `implicit` TLS, or `none`; AUTH PLAIN when a username is set; one connection per send; `Timeout` bounds the whole session and a canceled context aborts it.
- **`template.Registry`**: loads `*.txt` (`text/template`) and `*.html` (`html/template`, contextual escaping) from any `fs.FS`, typically an `embed.FS`; renders by name.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/lib/mailer
```

### Basic usage

```go
cfg := mailer.SMTPConfig{Host: "smtp.example.com", Username: "apikey", Password: os.Getenv("SMTP_PASSWORD")}
t, err := mailer.NewSMTPTransport(&cfg) // STARTTLS on 587, 30s timeout
if err != nil {
    return err
}
m := mailer.New(t)

err = m.Send(ctx, &mailer.Message{
    From:    "Acme <no-reply@acme.test>",
    To:      []string{"bia@example.com"},
    Subject: "Welcome",
    Text:    "Hello Bia",
    HTML:    "<p>Hello Bia</p>",
})
```

### YAML configuration

```yaml
mail:
  host: smtp.example.com
  port: 465
  username: apikey
  password: ${SMTP_PASSWORD}
  tls: implicit   # starttls (default) | implicit | none
  timeout: 10s
```

### Templates

```go
//go:embed templates/*.txt templates/*.html
var files embed.FS

sub, _ := fs.Sub(files, "templates")
reg, err := template.New(sub) // welcome.txt + welcome.html => "welcome"
m := mailer.New(t, mailer.WithTemplates(reg))

err = m.SendTemplate(ctx, &mailer.Message{
    From: "no-reply@acme.test", To: []string{"bia@example.com"}, Subject: "Welcome",
}, "welcome", map[string]string{"Name": "Bia"})
```

A service that renders its own bodies does not need the registry: set `Text`/`HTML` and call `Send`.

### Testing without SMTP

Any type with `Send(ctx, *mailer.Message) error` is a `Transport`, so a small fake recording messages is enough; a gomock `MockTransport` is in `github.com/biairmal/go-sdk/mocks/mailer`.

## Options

| Option | Description |
|--------|-------------|
| `WithTemplates(r *template.Registry)` | Registry `SendTemplate` renders from. Default: none (`SendTemplate` fails). |
| `WithTLSConfig(c *tls.Config)` | TLS client config for `SMTPTransport`, e.g. a private CA. `ServerName` defaults to `Host`. Default: system roots, TLS 1.2+. |

| `SMTPConfig` field | Default | Description |
|--------|---------|-------------|
| `host` | (required) | SMTP server; also the certificate name. |
| `port` | 587, or 465 for `implicit` | Server port. |
| `username` / `password` | empty | AUTH PLAIN when `username` is set. |
| `tls` | `starttls` | `starttls`, `implicit` or `none`. |
| `timeout` | `30s` | Bounds one whole send. |

## Limitations

- **One SMTP connection per message**: no pooling. Fine for transactional mail; a bulk sender wants a pooled transport.
- **No Cc, Bcc, Reply-To or attachments**: add them to `Message` when a consumer needs them.
- **AUTH PLAIN only** (stdlib `smtp.PlainAuth`), which `net/smtp` refuses over an unencrypted connection to anything but localhost.
- **No retry**: a failed send returns an `errorz.CodeBadGateway` error wrapping the cause (`*textproto.Error` carries the server's reply code); retry belongs to the caller, e.g. an outbox.

## Dependencies

None beyond the standard library. Uses `errorz` (foundational leaf).

## See also

- [errorz](../errorz/README.md) – the error type validation and delivery errors use.
- [mocks](../../mocks/README.md) – `mockmailer.NewMockTransport`.
