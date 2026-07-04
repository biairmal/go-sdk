# Validator Package

`validator` validates structs and single values against tag-driven rules, wrapping [go-playground/validator/v10](https://github.com/go-playground/validator) behind an SDK-native interface so validation failures surface as `errorz` errors instead of a third-party error type.

## Overview

Request handlers need a consistent way to reject malformed input with a clean, per-field error response. `validator` reads Go struct tags (default key `validate`) and translates any failure into an `*errorz.Error` with code `errorz.CodeBadRequest` and a `fields` meta map of human-readable messages, so `httpkit` maps it to a 400 with no extra wiring. The backend is swappable behind the `Validator` interface; consumers never import go-playground directly.

## Features

- **Struct-tag validation**: `ValidateStruct(s any)` validates a value against its `validate` tags (or a custom tag name).
- **Single-value validation**: `ValidateVar(field any, tag string)` validates one value against a tag expression, e.g. `"required,email"`.
- **Custom rules**: `Register` (runtime) or `WithCustomValidation` (construction-time) add project-specific validation functions without leaking the go-playground types.
- **`errorz`-native errors**: field failures become `errorz.BadRequest()` with per-field messages under `Meta["fields"]`; non-validation failures (e.g. passing a non-struct) become `errorz.CodeInternal`.
- **Config-driven field names**: `FieldNameTag` (e.g. `"json"`) makes error field names match the wire format instead of the Go field name.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/validator
```

### Basic usage

```go
package main

import (
    "fmt"

    "github.com/biairmal/go-sdk/validator"
)

type CreateUser struct {
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age"   validate:"gte=0,lte=130"`
}

func main() {
    v := validator.New(validator.Config{TagName: "validate", FieldNameTag: "json"})

    err := v.ValidateStruct(CreateUser{Email: "not-an-email", Age: 30})
    fmt.Println(err) // *errorz.Error, code ERR_BAD_REQUEST, Meta["fields"]["email"] = "..."
}
```

### In an HTTP handler

```go
func handler(v validator.Validator) httpkit.HandlerFunc {
    return func(r *http.Request) (any, error) {
        var body CreateUser
        if err := serializer.ParseJSON(r.Body, &body); err != nil {
            return nil, err
        }
        if err := v.ValidateStruct(body); err != nil {
            return nil, err // httpkit maps errorz.CodeBadRequest to HTTP 400
        }
        // ... happy path
        return body, nil
    }
}
```

### Custom validation rules

```go
v := validator.New(validator.Config{},
    validator.WithCustomValidation("is-slug", func(fl validator.FieldLevel) bool {
        return slugPattern.MatchString(fl.Field().String())
    }),
)

// or register after construction:
_ = v.Register("is-slug", func(fl validator.FieldLevel) bool {
    return slugPattern.MatchString(fl.Field().String())
})
```

## Options

| Option | Description |
|--------|-------------|
| `WithCustomValidation(tag string, fn func(FieldLevel) bool)` | Registers a custom validation rule at construction time (a func — can't live in YAML). Blank `tag` or nil `fn` is ignored. |

## Limitations

- **No i18n**: field messages are English-only, built from the tag and its parameter; translate at a higher layer if needed.
- **Config errors fall back silently**: `New` has no error return (it's a pure constructor), so an invalid `Config` (e.g. a `TagName` containing whitespace) falls back to `DefaultConfig()` rather than failing construction. Call `Config.Validate()` yourself first if you need to surface a config error.

## Dependencies

- [github.com/go-playground/validator/v10](https://github.com/go-playground/validator) – struct-tag validation engine.

## See also

- [errorz](../errorz/README.md) – the error type validation failures are translated into.
- [httpkit](../httpkit/README.md) – maps `errorz.CodeBadRequest` to HTTP 400 at the edge; pair `ValidateStruct` with request handlers.
- [serializer](../serializer/README.md) – `ParseJSON` typically runs just before `ValidateStruct` in a handler.
