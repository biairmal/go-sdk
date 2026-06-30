# Serializer Package

Thin wrappers around `encoding/json` for marshalling Go values to JSON bytes and unmarshalling JSON bytes back into Go values.

## Overview

The serializer package provides two functions — `ToJSON` and `ParseJSON` — that delegate directly to the Go standard library. It exists as a named boundary in the SDK so that callers can import a single, consistent symbol rather than sprinkling `encoding/json` calls throughout application code. Because the package is stateless and has no third-party dependencies, it is safe to import from any SDK layer.

## Features

- **JSON marshalling**: Converts any marshallable Go value to `[]byte` via `ToJSON`.
- **JSON unmarshalling**: Parses JSON bytes into a Go value via `ParseJSON`.
- **Zero dependencies**: Wraps only the standard library — no third-party modules added to your build.
- **Transparent errors**: Propagates `encoding/json` errors unchanged so callers can inspect them with `errors.As`.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/serializer
```

### Basic usage

```go
package main

import (
    "fmt"
    "github.com/biairmal/go-sdk/serializer"
)

type User struct {
    Name string `json:"name"`
    Age  int    `json:"age"`
}

func main() {
    u := User{Name: "alice", Age: 30}

    data, err := serializer.ToJSON(u)
    if err != nil {
        panic(err)
    }
    fmt.Println(string(data)) // {"name":"alice","age":30}

    var decoded User
    if err := serializer.ParseJSON(data, &decoded); err != nil {
        panic(err)
    }
    fmt.Printf("%+v\n", decoded) // {Name:alice Age:30}
}
```

## Limitations

- **No streaming support**: Both functions work with `[]byte` only. For large payloads use `encoding/json` directly with `json.NewEncoder` / `json.NewDecoder`.
- **No custom options**: There is no way to configure indentation, field ordering, or other marshal options. Use `encoding/json` directly if you need that control.
- **Standard-library error types**: Returned errors are `*json.SyntaxError`, `*json.UnmarshalTypeError`, etc. — the same types the standard library returns. Wrap them with `errorz` at the call site if you need SDK-typed errors.

## See also

- [encoding/json](https://pkg.go.dev/encoding/json) – standard library package this wraps.
- [errorz](../errorz/README.md) – SDK-wide structured error type; wrap serializer errors here at the use-case layer.
