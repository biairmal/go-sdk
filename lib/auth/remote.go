package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"time"

	"github.com/biairmal/go-sdk/lib/errorz"
)

// maxRemoteBytes caps the remote verification response body read.
const maxRemoteBytes = 1 << 20

// Token-forward locations for the remote validator request.
const (
	ForwardInHeader = "header"
	ForwardInBody   = "body"
	ForwardInQuery  = "query"
)

// Remote-validator defaults.
const (
	defaultRemoteMethod  = "POST"
	defaultRemoteTimeout = 5 * time.Second
	defaultForwardName   = "Authorization"
	defaultForwardPrefix = "Bearer "
	defaultSubjectField  = "sub"
	defaultRolesField    = "roles"
)

// TokenForward controls how the incoming token is attached to the verification
// request the remote validator makes to the identity service.
type TokenForward struct {
	// In is where the token is placed: "header", "body", or "query".
	In string `mapstructure:"in"`
	// Name is the header name, JSON body field, or query parameter name.
	Name string `mapstructure:"name"`
	// Prefix is prepended to the token value (e.g. "Bearer "), typically for headers.
	Prefix string `mapstructure:"prefix"`
}

// ClaimsMapping maps an arbitrary JSON verification response onto Claims using
// dotted-path field lookups.
type ClaimsMapping struct {
	// ClaimsPath is the dotted path to the object holding the claims; "" means
	// the response root.
	ClaimsPath string `mapstructure:"claims_path"`
	// SubjectField is the dotted path (within the claims object) to the subject.
	SubjectField string `mapstructure:"subject_field"`
	// RolesField is the dotted path (within the claims object) to the roles.
	RolesField string `mapstructure:"roles_field"`
	// ActiveField, when set, is a dotted path to a boolean that must be true
	// for the token to be accepted.
	ActiveField string `mapstructure:"active_field"`
}

// RemoteConfig configures the remote (network) validator.
type RemoteConfig struct {
	// URL is the identity service verification endpoint.
	URL string `mapstructure:"url"`
	// Method is the HTTP method used; defaults to POST.
	Method string `mapstructure:"method"`
	// Forward controls how the token is sent.
	Forward TokenForward `mapstructure:"forward"`
	// Mapping maps the response JSON onto Claims.
	Mapping ClaimsMapping `mapstructure:"mapping"`
	// Timeout bounds each verification request; defaults to 5s.
	Timeout time.Duration `mapstructure:"timeout"`
}

// remoteValidator verifies tokens by calling an external identity service and
// mapping its JSON response onto Claims.
type remoteValidator struct {
	cfg    RemoteConfig
	client *http.Client
}

// NewRemote returns a Validator that verifies tokens against an identity
// service. Zero-valued config fields fall back to sensible defaults.
func NewRemote(cfg *RemoteConfig, opts ...Option) (Validator, error) {
	if cfg.URL == "" {
		return nil, errorz.BadRequest().WithMessage("auth: remote.url is required")
	}
	resolved := *cfg
	remoteWithDefaults(&resolved)
	s := applyOptions(opts...)
	client := s.httpClient
	if client == nil {
		client = defaultHTTPClient()
	}
	return &remoteValidator{cfg: resolved, client: client}, nil
}

// Validate calls the identity service and maps its response onto Claims.
func (v *remoteValidator) Validate(ctx context.Context, token string) (Claims, error) {
	ctx, cancel := context.WithTimeout(ctx, v.cfg.Timeout)
	defer cancel()

	req, err := v.buildRequest(ctx, token)
	if err != nil {
		return nil, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, errorz.Wrap(err).WithCode(errorz.CodeBadGateway).WithMessage("auth: remote verify request failed")
	}
	defer func() { _ = resp.Body.Close() }()

	if err := statusError(resp.StatusCode); err != nil {
		return nil, err
	}
	return v.decodeClaims(resp.Body)
}

// buildRequest constructs the verification request, attaching the token per the
// configured TokenForward (header, body, or query).
func (v *remoteValidator) buildRequest(ctx context.Context, token string) (*http.Request, error) {
	value := v.cfg.Forward.Prefix + token
	target := v.cfg.URL
	var body io.Reader
	contentType := ""

	switch v.cfg.Forward.In {
	case ForwardInQuery:
		target = appendQuery(target, v.cfg.Forward.Name, value)
	case ForwardInBody:
		payload, err := json.Marshal(map[string]string{v.cfg.Forward.Name: value})
		if err != nil {
			return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("auth: encode remote body")
		}
		body = bytes.NewReader(payload)
		contentType = "application/json"
	}

	req, err := http.NewRequestWithContext(ctx, v.cfg.Method, target, body)
	if err != nil {
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("auth: build remote request")
	}
	if v.cfg.Forward.In == ForwardInHeader {
		req.Header.Set(v.cfg.Forward.Name, value)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req, nil
}

// decodeClaims maps the identity service's JSON response onto Claims using the
// configured dotted-path field mapping.
func (v *remoteValidator) decodeClaims(r io.Reader) (Claims, error) {
	var raw map[string]any
	if err := json.NewDecoder(io.LimitReader(r, maxRemoteBytes)).Decode(&raw); err != nil {
		return nil, errorz.Wrap(err).WithCode(errorz.CodeBadGateway).WithMessage("auth: decode remote response")
	}

	claimsObj, err := resolveClaimsObject(raw, v.cfg.Mapping.ClaimsPath)
	if err != nil {
		return nil, err
	}
	if m := v.cfg.Mapping; m.ActiveField != "" {
		active, _ := getPath(claimsObj, m.ActiveField)
		if b, ok := active.(bool); !ok || !b {
			return nil, unauthorized(ErrTokenInactive)
		}
	}

	subjectVal, _ := getPath(claimsObj, v.cfg.Mapping.SubjectField)
	subject := asString(subjectVal)
	if subject == "" {
		return nil, unauthorized(ErrInvalidToken)
	}
	var roles []string
	if rolesVal, ok := getPath(claimsObj, v.cfg.Mapping.RolesField); ok {
		roles = toStringSlice(rolesVal)
	}
	return newMappedClaims(claimsObj, subject, roles), nil
}

// resolveClaimsObject returns the object holding the claims, walking claimsPath
// when set (empty path means the response root).
func resolveClaimsObject(raw map[string]any, claimsPath string) (map[string]any, error) {
	if claimsPath == "" {
		return raw, nil
	}
	nested, ok := getPath(raw, claimsPath)
	if !ok {
		return nil, unauthorized(ErrInvalidToken)
	}
	obj, ok := nested.(map[string]any)
	if !ok {
		return nil, unauthorized(ErrInvalidToken)
	}
	return obj, nil
}

// statusError maps a verification HTTP status to a result: 2xx is success,
// 401/403 is a bad credential, and anything else is an upstream outage (502).
func statusError(code int) error {
	switch {
	case code >= 200 && code < 300:
		return nil
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return unauthorized(ErrInvalidToken)
	default:
		return errorz.BadGateway().WithMessage(fmt.Sprintf("auth: remote verify returned %d", code))
	}
}

// appendQuery adds key=value to a URL's query string.
func appendQuery(rawURL, key, value string) string {
	u, err := neturl.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	q.Set(key, value)
	u.RawQuery = q.Encode()
	return u.String()
}

// defaultHTTPClient is the fallback client for JWKS and remote requests.
func defaultHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}
