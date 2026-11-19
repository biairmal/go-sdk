package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func reqFrom(remote string, headers map[string]string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.RemoteAddr = remote
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestIPResolver_ClientIP(t *testing.T) {
	tests := []struct {
		name    string
		trusted []string
		remote  string
		headers map[string]string
		want    string
	}{
		{name: "direct peer, port stripped", remote: "203.0.113.9:5555", want: "203.0.113.9"},
		{
			name: "forged X-Forwarded-For ignored", remote: "203.0.113.9:5555",
			headers: map[string]string{"X-Forwarded-For": "1.2.3.4"}, want: "203.0.113.9",
		},
		{
			name: "forged X-Real-IP ignored", remote: "203.0.113.9:5555",
			headers: map[string]string{"X-Real-IP": "1.2.3.4"}, want: "203.0.113.9",
		},
		{name: "ipv6 peer", remote: "[2001:db8::1]:443", want: "2001:db8::1"},
		{name: "ipv4-mapped peer", remote: "[::ffff:203.0.113.9]:80", want: "203.0.113.9"},
		{name: "bare host", remote: "203.0.113.9", want: "203.0.113.9"},
		{
			name: "trusted proxy, header believed", trusted: []string{"10.0.0.0/8"}, remote: "10.0.0.5:1",
			headers: map[string]string{"X-Forwarded-For": "198.51.100.7"}, want: "198.51.100.7",
		},
		{
			name: "trusted proxy, left entries forged", trusted: []string{"10.0.0.0/8"}, remote: "10.0.0.5:1",
			headers: map[string]string{"X-Forwarded-For": "6.6.6.6, 198.51.100.7, 10.0.0.9"}, want: "198.51.100.7",
		},
		{
			name: "trusted proxy, X-Real-IP", trusted: []string{"10.0.0.0/8"}, remote: "10.0.0.5:1",
			headers: map[string]string{"X-Real-IP": "198.51.100.7"}, want: "198.51.100.7",
		},
		{name: "trusted proxy, no header", trusted: []string{"10.0.0.0/8"}, remote: "10.0.0.5:1", want: "10.0.0.5"},
		{
			name: "trusted proxy, junk entry", trusted: []string{"10.0.0.0/8"}, remote: "10.0.0.5:1",
			headers: map[string]string{"X-Forwarded-For": "not-an-ip"}, want: "10.0.0.5",
		},
		{
			name: "untrusted peer sends header", trusted: []string{"10.0.0.0/8"}, remote: "203.0.113.9:1",
			headers: map[string]string{"X-Forwarded-For": "1.2.3.4"}, want: "203.0.113.9",
		},
		{
			name: "single address trusted", trusted: []string{"192.168.1.10"}, remote: "192.168.1.10:9",
			headers: map[string]string{"X-Forwarded-For": "198.51.100.7"}, want: "198.51.100.7",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ir, err := NewIPResolver(tt.trusted...)
			if err != nil {
				t.Fatal(err)
			}
			if got := ir.ClientIP(reqFrom(tt.remote, tt.headers)); got != tt.want {
				t.Errorf("ClientIP = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIPResolver_NilTrustsNobody(t *testing.T) {
	var ir *IPResolver
	got := ir.ClientIP(reqFrom("203.0.113.9:1", map[string]string{"X-Forwarded-For": "1.2.3.4"}))
	if got != "203.0.113.9" {
		t.Errorf("ClientIP = %q, want the peer", got)
	}
}

func TestNewIPResolver_Invalid(t *testing.T) {
	if _, err := NewIPResolver("10.0.0.0/8", "nope"); err == nil {
		t.Error("want an error for an entry that is neither a CIDR nor an IP")
	}
}

// Two connections from one host differ only in the source port; they must share one rate-limit key.
func TestKeyByIP_SameHostSameKey(t *testing.T) {
	a := KeyByIP(reqFrom("203.0.113.9:1111", nil))
	b := KeyByIP(reqFrom("203.0.113.9:2222", nil))
	if a != b {
		t.Errorf("keys differ across ports: %q vs %q", a, b)
	}
}

// A client that rotates X-Forwarded-For must not get a fresh bucket each time.
func TestKeyByIP_ForgedHeaderSameKey(t *testing.T) {
	a := KeyByIP(reqFrom("203.0.113.9:1", map[string]string{"X-Forwarded-For": "1.1.1.1"}))
	b := KeyByIP(reqFrom("203.0.113.9:1", map[string]string{"X-Forwarded-For": "2.2.2.2"}))
	if a != b {
		t.Errorf("keys differ with a forged header: %q vs %q", a, b)
	}
}
