package sqlkit

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// parseLibpq reads a libpq key=value connection string the way lib/pq and pgx do: values are literal (never
// URL-decoded), may be single-quoted, and inside quotes a backslash escapes the next character.
func parseLibpq(t *testing.T, dsn string) map[string]string {
	t.Helper()
	out := map[string]string{}
	i := 0
	for i < len(dsn) {
		for i < len(dsn) && dsn[i] == ' ' {
			i++
		}
		if i >= len(dsn) {
			break
		}
		eq := strings.IndexByte(dsn[i:], '=')
		if eq < 0 {
			t.Fatalf("no '=' in %q at %d", dsn, i)
		}
		key := dsn[i : i+eq]
		i += eq + 1
		var val strings.Builder
		if i < len(dsn) && dsn[i] == '\'' {
			i++
			for {
				if i >= len(dsn) {
					t.Fatalf("unterminated quote in %q", dsn)
				}
				if dsn[i] == '\\' && i+1 < len(dsn) {
					val.WriteByte(dsn[i+1])
					i += 2
					continue
				}
				if dsn[i] == '\'' {
					i++
					break
				}
				val.WriteByte(dsn[i])
				i++
			}
		} else {
			for i < len(dsn) && dsn[i] != ' ' {
				val.WriteByte(dsn[i])
				i++
			}
		}
		out[key] = val.String()
	}
	return out
}

func TestDSN_PostgresRoundTripsAwkwardValues(t *testing.T) {
	passwords := []string{
		"plain", "p@ss word", "100%", "a+b=c", "it's", `back\slash`, `'; DROP`, "tab\tx", "#hash", "ünï", "",
	}
	for _, pw := range passwords {
		t.Run(pw, func(t *testing.T) {
			c := DBConfig{
				Driver: "postgres", Host: "db.local", Port: 5433, Database: "my db", Username: "o'brien",
				Password: pw, SSLMode: SSLModeRequire, ConnectTimeout: 7 * time.Second,
			}
			got := parseLibpq(t, c.DSN())
			want := map[string]string{
				"host": "db.local", "port": "5433", "user": "o'brien", "password": pw,
				"dbname": "my db", "sslmode": "require", "connect_timeout": "7",
			}
			if len(got) != len(want) {
				t.Fatalf("parsed %v, want %v", got, want)
			}
			for k, v := range want {
				if got[k] != v {
					t.Errorf("%s = %q, want %q (dsn %q)", k, got[k], v, c.DSN())
				}
			}
		})
	}
}

// A host or database value must never be able to add a second parameter to the connection string.
func TestDSN_PostgresValueCannotInjectParameters(t *testing.T) {
	c := DBConfig{
		Driver: "postgres", Host: "h sslmode=disable", Port: 5432, Database: "d", Username: "u", Password: "p",
		SSLMode: SSLModeVerifyFull,
	}
	got := parseLibpq(t, c.DSN())
	if got["sslmode"] != "verify-full" || got["host"] != "h sslmode=disable" {
		t.Errorf("parsed %v, want sslmode verify-full and the host kept whole", got)
	}
}

func TestDSN_PostgresOmitsEmptySSLMode(t *testing.T) {
	c := DBConfig{Driver: "postgres", Host: "h", Port: 1, Database: "d", Username: "u", Password: "p"}
	if strings.Contains(c.DSN(), "sslmode") {
		t.Errorf("dsn %q has sslmode, want the driver default when ssl_mode is empty", c.DSN())
	}
}

const mysqlBase = "u:p@ss word%/!@tcp(h:3306)/d?parseTime=true&timeout=5s"

func TestDSN_MySQL(t *testing.T) {
	tests := []struct {
		name    string
		sslMode string
		want    string
	}{
		{name: "no ssl mode", want: mysqlBase},
		{name: "disable", sslMode: SSLModeDisable, want: mysqlBase + "&tls=false"},
		{name: "prefer", sslMode: SSLModePrefer, want: mysqlBase + "&tls=preferred"},
		{name: "require", sslMode: SSLModeRequire, want: mysqlBase + "&tls=skip-verify"},
		{name: "verify-ca", sslMode: SSLModeVerifyCA, want: mysqlBase + "&tls=true"},
		{name: "verify-full", sslMode: SSLModeVerifyFull, want: mysqlBase + "&tls=true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := DBConfig{
				Driver: "mysql", Host: "h", Port: 3306, Database: "d", Username: "u",
				Password: "p@ss word%/!", SSLMode: tt.sslMode,
			}
			if got := c.DSN(); got != tt.want {
				t.Errorf("DSN = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigValidate_SSLMode(t *testing.T) {
	base := func() Config {
		return Config{Leader: DBConfig{Driver: "postgres", Host: "h", Database: "d"}}
	}
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{name: "empty is the driver default", mutate: func(*Config) {}},
		{name: "valid", mutate: func(c *Config) { c.Leader.SSLMode = SSLModeVerifyFull }},
		{name: "typo", mutate: func(c *Config) { c.Leader.SSLMode = "verify_full" }, wantErr: true},
		{name: "case matters", mutate: func(c *Config) { c.Leader.SSLMode = "Require" }, wantErr: true},
		{name: "bad follower", mutate: func(c *Config) { c.Followers = []DBConfig{{SSLMode: "nope"}} }, wantErr: true},
		{name: "good follower", mutate: func(c *Config) { c.Followers = []DBConfig{{SSLMode: SSLModeRequire}} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("err = %v, want it to wrap ErrInvalidConfig", err)
			}
		})
	}
}
