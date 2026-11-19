package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/biairmal/go-sdk/lib/ctxkit"
	"github.com/biairmal/go-sdk/lib/errorz"
)

// IPResolver finds the address of the client that made a request.
//
// Forwarding headers (X-Forwarded-For, X-Real-IP) are written by the caller, so they can say anything. An
// IPResolver believes them only when the TCP peer is one of the trusted proxies it was built with; for every
// other peer it returns the peer's own address. A nil *IPResolver trusts no proxy, which is the right setting
// for a service that clients reach directly.
type IPResolver struct {
	trusted []netip.Prefix
}

// NewIPResolver builds a resolver that trusts forwarding headers from peers inside trustedProxies. Each entry is
// a CIDR ("10.0.0.0/8", "fd00::/8") or a single address ("192.168.1.10"). With no entries it trusts nobody.
func NewIPResolver(trustedProxies ...string) (*IPResolver, error) {
	r := &IPResolver{}
	for _, s := range trustedProxies {
		s = strings.TrimSpace(s)
		if p, err := netip.ParsePrefix(s); err == nil {
			r.trusted = append(r.trusted, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(s)
		if err != nil {
			return nil, errorz.BadRequest().WithMessage("httpkit: trusted proxy must be a CIDR or an IP address")
		}
		a = a.Unmap()
		r.trusted = append(r.trusted, netip.PrefixFrom(a, a.BitLen()))
	}
	return r, nil
}

// ClientIP returns the client address for r: the TCP peer, unless the peer is a trusted proxy, in which case
// the nearest address in X-Forwarded-For (read right to left) that is not itself a trusted proxy, or X-Real-IP
// when there is no X-Forwarded-For. It never returns the port.
func (ir *IPResolver) ClientIP(r *http.Request) string {
	peer, ok := peerAddr(r)
	if !ok {
		return r.RemoteAddr
	}
	if !ir.isTrusted(peer) {
		return peer.String()
	}
	if fwd := strings.Join(r.Header.Values("X-Forwarded-For"), ","); fwd != "" {
		parts := strings.Split(fwd, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			a, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
			if err != nil {
				return peer.String() // not an address a proxy would write: believe nothing in the header
			}
			if a = a.Unmap(); !ir.isTrusted(a) {
				return a.String()
			}
		}
		return peer.String()
	}
	if a, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("X-Real-IP"))); err == nil {
		return a.Unmap().String()
	}
	return peer.String()
}

// KeyByIP is the package-level KeyByIP using this resolver.
func (ir *IPResolver) KeyByIP(r *http.Request) string { return ir.ClientIP(r) }

// KeyByUser is the package-level KeyByUser using this resolver.
func (ir *IPResolver) KeyByUser(r *http.Request) string {
	if id := ctxkit.UserID(r.Context()); id != "" {
		return id
	}
	return ir.ClientIP(r)
}

// KeyByHeader is the package-level KeyByHeader using this resolver for the fallback.
func (ir *IPResolver) KeyByHeader(name string) KeyFunc {
	return func(r *http.Request) string {
		if v := r.Header.Get(name); v != "" {
			return v
		}
		return ir.ClientIP(r)
	}
}

func (ir *IPResolver) isTrusted(a netip.Addr) bool {
	if ir == nil {
		return false
	}
	for _, p := range ir.trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// peerAddr parses the host part of r.RemoteAddr.
func peerAddr(r *http.Request) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr // some test servers and unix sockets give a bare host
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return a.Unmap().WithZone(""), true
}
