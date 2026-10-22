package authz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
)

const (
	// rolePermissionsPath is identity's internal role map endpoint.
	rolePermissionsPath = "/v1/internal/roles/permissions"
	// maxStaleFactor bounds how long a stale role map is served while identity
	// is unreachable: up to this many cache TTLs since the last good fetch.
	maxStaleFactor = 10
	// maxRoleMapBytes caps the role map response body read.
	maxRoleMapBytes = 1 << 20
	// defaultHTTPTimeout bounds a role map fetch when no client is supplied.
	defaultHTTPTimeout = 5 * time.Second
)

// identitySource maps roles with a per-tenant role map fetched from identity.
type identitySource struct {
	a *authorizer

	mu      sync.Mutex
	tenants map[string]*tenantRoles
}

// tenantRoles is one tenant's cached role map. Its own mutex serializes
// refreshes for that tenant without blocking other tenants.
type tenantRoles struct {
	mu        sync.Mutex
	roles     map[string][]string
	fetchedAt time.Time
}

// roleMapResponse is identity's response envelope for the role map.
type roleMapResponse struct {
	Data struct {
		Roles map[string][]string `json:"roles"`
	} `json:"data"`
}

// newIdentitySource builds the identity source for a.
func newIdentitySource(a *authorizer) *identitySource {
	if a.httpClient == nil {
		a.httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &identitySource{a: a, tenants: map[string]*tenantRoles{}}
}

// callerPerms maps the caller's roles in the caller's tenant.
func (s *identitySource) callerPerms(ctx context.Context, c auth.Claims, tenantID string) ([]string, error) {
	return s.rolePerms(ctx, tenantID, c.Roles())
}

// rolePerms maps roles with the tenant's cached role map.
func (s *identitySource) rolePerms(ctx context.Context, tenantID string, roles []string) ([]string, error) {
	if tenantID == "" {
		return nil, errorz.Unauthorized().WithMessage("authz: token has no tenant")
	}
	m, err := s.roleMap(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return mapRoles(m, roles), nil
}

// roleMap returns the tenant's role map, refreshing it once it is older than
// the cache TTL. When a refresh fails, the previous map is served until it is
// older than maxStaleFactor TTLs; after that the error is returned.
func (s *identitySource) roleMap(ctx context.Context, tenantID string) (map[string][]string, error) {
	s.mu.Lock()
	t, ok := s.tenants[tenantID]
	if !ok {
		t = &tenantRoles{}
		s.tenants[tenantID] = t
	}
	s.mu.Unlock()

	t.mu.Lock()
	defer t.mu.Unlock()
	now := s.a.now()
	age := now.Sub(t.fetchedAt)
	if t.roles != nil && age < s.a.cfg.CacheTTL {
		return t.roles, nil
	}
	roles, err := s.fetch(ctx, tenantID)
	if err == nil {
		t.roles, t.fetchedAt = roles, now
		return roles, nil
	}
	if t.roles != nil && age < maxStaleFactor*s.a.cfg.CacheTTL {
		if s.a.log != nil {
			s.a.log.Warn("authz: serving stale role map", logger.F("tenant_id", tenantID), logger.F("error", err))
		}
		return t.roles, nil
	}
	return nil, err
}

// fetch loads the tenant's role map from identity.
func (s *identitySource) fetch(ctx context.Context, tenantID string) (map[string][]string, error) {
	u := strings.TrimRight(s.a.cfg.IdentityURL, "/") + rolePermissionsPath + "?tenant_id=" + url.QueryEscape(tenantID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("authz: build role map request")
	}
	if s.a.serviceToken != nil {
		tok, terr := s.a.serviceToken(ctx)
		if terr != nil {
			return nil, errorz.Wrap(terr).WithCode(errorz.CodeServiceUnavailable).
				WithMessage("authz: get service token")
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := s.a.httpClient.Do(req)
	if err != nil {
		return nil, errorz.Wrap(err).WithCode(errorz.CodeServiceUnavailable).WithMessage("authz: fetch role map")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, errorz.ServiceUnavailable().
			WithMessage(fmt.Sprintf("authz: role map endpoint returned %d", resp.StatusCode))
	}
	var body roleMapResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxRoleMapBytes)).Decode(&body); err != nil {
		return nil, errorz.Wrap(err).WithCode(errorz.CodeServiceUnavailable).WithMessage("authz: decode role map")
	}
	if body.Data.Roles == nil {
		body.Data.Roles = map[string][]string{}
	}
	return body.Data.Roles, nil
}
