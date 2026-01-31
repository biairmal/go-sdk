# Config Package

A Viper-based configuration loader for Go that supports JSON/YAML files, in-file environment variable substitution, and optional .env loading. The package populates application-defined structs (including nested structs) from one or more config files.

## Overview

The config package wraps [Viper](https://github.com/spf13/viper) to provide a simple `Load(dst, opts...)` API. It adds two behaviours that Viper does not provide out of the box: loading a `.env` file before config read, and substituting `${VAR}` and `${VAR:default_value}` inside config file content. Viper handles file format (JSON, YAML), merging multiple files, environment variable precedence, and unmarshalling into structs (including nested structs and types such as `time.Duration`).

## Features

- **Struct injection**: Define your own Go config struct; the package populates it from JSON or YAML.
- **Nested config**: Use nested structs (e.g. `Handler`, `Domain`) with `mapstructure` tags; Viper unmarshals nested keys.
- **Multiple files**: Pass several config file paths; they are merged in order (later files override overlapping keys).
- **In-file env substitution**: Use `${ENV_VAR}` or `${ENV_VAR:default_value}` in config file content; substitution runs before Viper parses the file.
- **.env loading**: Optionally load a `.env` file from a path (e.g. project root) so environment variables are set before substitution and Viper.
- **Duration**: Viper’s default decode hook supports string values like `60s` for `time.Duration` fields.
- **Remote config (future)**: Viper supports remote providers (Consul, etcd); the wrapper can expose or document that for later use.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/config
```

### Basic usage

```go
package main

import (
    "github.com/biairmal/go-sdk/config"
)

type AppConfig struct {
    Port int    `mapstructure:"port"`
    Name string `mapstructure:"name"`
}

func main() {
    var cfg AppConfig
    err := config.Load(&cfg,
        config.EnvFile(".env"),
        config.Files("config.yaml"),
    )
    if err != nil {
        panic(err)
    }
}
```

### Nested config (Handler, Domain, …)

Use nested structs and `mapstructure` tags so YAML/JSON keys map to fields. Viper uses [mapstructure](https://github.com/go-viper/mapstructure) for unmarshalling.

**config.yaml**:

```yaml
handler:
  port: 8080
  read_timeout: 60s
domain:
  name: example.com
  tls: true
database_url: ${DATABASE_URL:postgres://localhost/default}
```

**Go**:

```go
type HandlerOptions struct {
    Port        int           `mapstructure:"port"`
    ReadTimeout time.Duration `mapstructure:"read_timeout"`
}

type DomainOptions struct {
    Name string `mapstructure:"name"`
    TLS  bool   `mapstructure:"tls"`
}

type AppConfig struct {
    Handler     HandlerOptions `mapstructure:"handler"`
    Domain      DomainOptions  `mapstructure:"domain"`
    DatabaseURL string         `mapstructure:"database_url"`
}

var cfg AppConfig
err := config.Load(&cfg,
    config.EnvFile(".env"),
    config.Files("config.yaml"),
)
```

- **Key naming**: By default mapstructure uses lowercased field names. Use `mapstructure:"handler"` (and similar) to match YAML/JSON keys. Viper keys are case-insensitive.
- **Deep nesting**: Deeper structs work the same way (e.g. `handler.timeouts.connect` in YAML maps to `Handler.Timeouts.Connect` with the right tags).

### Environment variable substitution

Inside config file content you can use:

- **`${VAR}`**: Replaced with `os.Getenv("VAR")`. Empty if the variable is unset.
- **`${VAR:default_value}`**: Replaced with the value of `VAR` if set and non-empty; otherwise `default_value`.

Substitution runs on the raw file bytes before Viper parses the file, so any field type (string, number, nested) can receive substituted values.

Example:

```yaml
database_url: ${DATABASE_URL:postgres://localhost/default}
log_level: ${LOG_LEVEL:info}
```

### .env file

Use `config.EnvFile(path)` to load a `.env` file before config files are read. Path is relative to the current working directory or absolute. If the file does not exist, `Load` does not fail (optional .env). To fail when the file is missing, use `config.LoadEnvFile(path)` before `config.Load` and handle the error.

Typical usage when the application runs from the project root:

```go
config.Load(&cfg, config.EnvFile(".env"), config.Files("config.yaml"))
```

### Options

| Option | Description |
|--------|-------------|
| `EnvFile(path string)` | Path to a .env file to load before reading config. Empty means no .env. Missing file is ignored. |
| `Files(paths ...string)` | Config file paths in order. First file is base; later files merge over it (later keys override). |

### Duration and other types

Viper’s default mapstructure hook includes `StringToTimeDurationHookFunc()`, so `time.Duration` fields accept string values like `60s` in YAML or JSON. Other standard types (int, bool, nested structs) work via Viper/mapstructure; use `mapstructure` struct tags.

## Limitations

- **Merge behaviour**: Viper merges at key level. When merging multiple files, slices and maps are replaced entirely, not merged element-wise.
- **.env path**: Path is relative to the current working directory unless absolute. If the process runs from a different directory, the caller must pass the correct path (e.g. from a flag or env).
- **In-file substitution**: Substitution is a single pass over the full file content; very large files are read into memory.
- **Remote config**: Not exposed by the wrapper yet. Viper supports `AddRemoteProvider` and `ReadRemoteConfig` (e.g. Consul, etcd); this can be added as options or documented as an escape hatch.

## See also

- [Viper](https://github.com/spf13/viper) – configuration with fangs
- [mapstructure](https://github.com/go-viper/mapstructure) – decode generic map values into structs
