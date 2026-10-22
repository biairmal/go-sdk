package authz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/errorz"
)

func ctxWith(claims map[string]any) context.Context {
	return auth.ContextWithClaims(context.Background(), auth.NewClaims(claims))
}

func TestMatches(t *testing.T) {
	tests := []struct {
		name   string
		grants []string
		perm   string
		want   bool
	}{
		{"exact", []string{"notes:read"}, "notes:read", true},
		{"other key", []string{"notes:read"}, "notes:write", false},
		{"star", []string{"*"}, "anything:at:all", true},
		{"prefix wildcard", []string{"notes:*"}, "notes:read", true},
		{"prefix wildcard nested", []string{"notes:*"}, "notes:read:any", true},
		{"prefix wildcard other resource", []string{"notes:*"}, "notesx:read", false},
		{"any does not imply plain", []string{"notes:read:any"}, "notes:read", false},
		{"wildcard in check refused", []string{"*"}, "notes:*", false},
		{"empty check refused", []string{"*"}, "", false},
		{"no grants", nil, "notes:read", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Matches(tt.grants, tt.perm); got != tt.want {
				t.Errorf("Matches(%v, %q) = %v, want %v", tt.grants, tt.perm, got, tt.want)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"defaults", DefaultConfig(), false},
		{"config source", withDefaults(Config{Source: SourceConfig}), false},
		{"identity without url", withDefaults(Config{Source: SourceIdentity}), true},
		{"identity with url", withDefaults(Config{Source: SourceIdentity, IdentityURL: "http://id"}), false},
		{"unknown source", withDefaults(Config{Source: "ldap"}), true},
		{"negative ttl", withDefaults(Config{CacheTTL: -time.Second}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRequireTokenAndConfigSources(t *testing.T) {
	roles := map[string][]string{"admin": {"*"}, "member": {"notes:read"}}
	tokenAZ, _ := New(&Config{Source: SourceToken})
	configAZ, _ := New(&Config{Source: SourceConfig, Roles: roles})
	perms := func(p ...any) context.Context { return ctxWith(map[string]any{"sub": "u", "perms": p}) }
	withRoles := func(r ...any) context.Context { return ctxWith(map[string]any{"sub": "u", "roles": r}) }
	forbidden := errorz.ErrForbidden

	tests := []struct {
		name     string
		az       Authorizer
		ctx      context.Context
		perm     string
		wantCode error
	}{
		{"token: no claims", tokenAZ, context.Background(), "notes:read", errorz.ErrUnauthorized},
		{"token: granted", tokenAZ, perms("notes:read"), "notes:read", nil},
		{"token: wildcard", tokenAZ, perms("notes:*"), "notes:write", nil},
		{"token: missing", tokenAZ, perms("notes:read"), "notes:write", forbidden},
		{"token: no perms claim", tokenAZ, ctxWith(map[string]any{"sub": "u"}), "notes:read", forbidden},
		{"token: super admin", tokenAZ, ctxWith(map[string]any{"sub": "u", "sa": true}), "tenants:manage", nil},
		{"token: sa false", tokenAZ, ctxWith(map[string]any{"sub": "u", "sa": false}), "tenants:manage", forbidden},
		{"config: member", configAZ, withRoles("member"), "notes:read", nil},
		{"config: member missing", configAZ, withRoles("member"), "notes:write", forbidden},
		{"config: admin star", configAZ, withRoles("admin"), "notes:write", nil},
		{"config: unknown role", configAZ, withRoles("ghost"), "notes:read", forbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.az.Require(tt.ctx, tt.perm)
			if tt.wantCode == nil {
				if err != nil {
					t.Fatalf("Require() = %v, want nil", err)
				}
				if !tt.az.Can(tt.ctx, tt.perm) {
					t.Error("Can() = false, want true")
				}
				return
			}
			if !errors.Is(err, tt.wantCode) {
				t.Fatalf("Require() = %v, want %v", err, tt.wantCode)
			}
			if tt.az.Can(tt.ctx, tt.perm) {
				t.Error("Can() = true, want false")
			}
		})
	}
}

func TestPermissions(t *testing.T) {
	az, _ := New(&Config{Source: SourceConfig, Roles: map[string][]string{"a": {"x:read"}, "b": {"y:read"}}})
	got := az.Permissions(ctxWith(map[string]any{"sub": "u", "roles": []any{"a", "b"}}))
	if len(got) != 2 || got[0] != "x:read" || got[1] != "y:read" {
		t.Errorf("Permissions() = %v, want union of both roles", got)
	}
	if got := az.Permissions(ctxWith(map[string]any{"sub": "u", "sa": true})); len(got) != 1 || got[0] != "*" {
		t.Errorf("super admin Permissions() = %v, want [*]", got)
	}
	if got := az.Permissions(context.Background()); got != nil {
		t.Errorf("no-claims Permissions() = %v, want nil", got)
	}
}

// identityServer serves a role map and counts fetches; fail switches it to 503.
type identityServer struct {
	fetches atomic.Int32
	fail    atomic.Bool
	srv     *httptest.Server
}

func newIdentityServer(t *testing.T) *identityServer {
	t.Helper()
	s := &identityServer{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.fetches.Add(1)
		if s.fail.Load() || r.URL.Path != rolePermissionsPath || r.URL.Query().Get("tenant_id") != "t1" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"timestamp":"2026-10-22T00:00:00Z","data":{"tenant_id":"t1",` +
			`"roles":{"member":["notes:read"]},"version":1}}`))
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func TestIdentitySourceCacheAndStale(t *testing.T) {
	ids := newIdentityServer(t)
	now := time.Unix(1_700_000_000, 0)
	az, err := New(&Config{Source: SourceIdentity, IdentityURL: ids.srv.URL, CacheTTL: time.Minute},
		WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := ctxWith(map[string]any{"sub": "u", "tid": "t1", "roles": []any{"member"}})

	steps := []struct {
		name        string
		advance     time.Duration
		down        bool
		wantErr     error
		wantFetches int32
	}{
		{"first call fetches", 0, false, nil, 1},
		{"fresh cache hit", 30 * time.Second, false, nil, 1},
		{"expired refetches", 31 * time.Second, false, nil, 2},
		{"down within stale window serves stale", 5 * time.Minute, true, nil, 3},
		{"down past 10 TTL denies", 6 * time.Minute, true, errorz.ErrServiceUnavailable, 4},
		{"recovers", 0, false, nil, 5},
	}
	for _, st := range steps {
		now = now.Add(st.advance)
		ids.fail.Store(st.down)
		err := az.Require(ctx, "notes:read")
		if st.wantErr == nil && err != nil {
			t.Fatalf("%s: Require() = %v, want nil", st.name, err)
		}
		if st.wantErr != nil && !errors.Is(err, st.wantErr) {
			t.Fatalf("%s: Require() = %v, want %v", st.name, err, st.wantErr)
		}
		if got := ids.fetches.Load(); got != st.wantFetches {
			t.Fatalf("%s: fetches = %d, want %d", st.name, got, st.wantFetches)
		}
	}
}

func TestIdentitySourceNoTenant(t *testing.T) {
	ids := newIdentityServer(t)
	az, _ := New(&Config{Source: SourceIdentity, IdentityURL: ids.srv.URL})
	err := az.Require(ctxWith(map[string]any{"sub": "u", "roles": []any{"member"}}), "notes:read")
	if !errors.Is(err, errorz.ErrUnauthorized) {
		t.Errorf("Require() = %v, want unauthorized", err)
	}
}

func TestIdentitySourceSendsServiceToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"data":{"roles":{}}}`))
	}))
	t.Cleanup(srv.Close)
	az, _ := New(&Config{Source: SourceIdentity, IdentityURL: srv.URL},
		WithServiceToken(func(context.Context) (string, error) { return "svc", nil }))
	_ = az.Require(ctxWith(map[string]any{"sub": "u", "tid": "t1"}), "notes:read")
	if gotAuth != "Bearer svc" {
		t.Errorf("Authorization = %q, want Bearer svc", gotAuth)
	}
}

type fakeResolver struct {
	roles []string
	err   error
	calls int
}

func (f *fakeResolver) RolesIn(context.Context, Scope, string) ([]string, error) {
	f.calls++
	return f.roles, f.err
}

func TestRequireIn(t *testing.T) {
	roles := map[string][]string{"member": {"events:read"}, "staff": {"guests:checkin"}}
	scope := Scope{Type: "event", ID: "e1"}
	member := ctxWith(map[string]any{"sub": "u", "roles": []any{"member"}})

	tests := []struct {
		name      string
		resolver  *fakeResolver
		perm      string
		wantErr   error
		wantCalls int
	}{
		{"tenant grant wins without resolver call", &fakeResolver{roles: []string{"staff"}}, "events:read", nil, 0},
		{"scoped grant allows", &fakeResolver{roles: []string{"staff"}}, "guests:checkin", nil, 1},
		{"resolver error is 500", &fakeResolver{err: errors.New("db down")}, "guests:checkin", errorz.ErrInternal, 1},
		{"no scoped role is 403", &fakeResolver{}, "guests:checkin", errorz.ErrForbidden, 1},
		{
			"scoped role without perm is 403", &fakeResolver{roles: []string{"member"}}, "guests:checkin",
			errorz.ErrForbidden, 1,
		},
		{"no resolver acts like Require", nil, "guests:checkin", errorz.ErrForbidden, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []Option
			if tt.resolver != nil {
				opts = append(opts, WithScopeResolver(tt.resolver))
			}
			az, _ := New(&Config{Source: SourceConfig, Roles: roles}, opts...)
			err := az.RequireIn(member, scope, tt.perm)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("RequireIn() = %v, want nil", err)
			}
			if tt.wantErr != nil && !sameCode(err, tt.wantErr) {
				t.Fatalf("RequireIn() = %v, want %v", err, tt.wantErr)
			}
			if tt.resolver != nil && tt.resolver.calls != tt.wantCalls {
				t.Errorf("resolver calls = %d, want %d", tt.resolver.calls, tt.wantCalls)
			}
		})
	}
}

func TestMemoResolvesOncePerRequest(t *testing.T) {
	ids := newIdentityServer(t)
	now := time.Unix(1_700_000_000, 0)
	az, _ := New(&Config{Source: SourceIdentity, IdentityURL: ids.srv.URL, CacheTTL: time.Nanosecond},
		WithClock(func() time.Time { now = now.Add(time.Second); return now }))
	ctx := WithMemo(ctxWith(map[string]any{"sub": "u", "tid": "t1", "roles": []any{"member"}}))
	for range 3 {
		_ = az.Can(ctx, "notes:read")
	}
	if got := ids.fetches.Load(); got != 1 {
		t.Errorf("fetches = %d, want 1 (memoised)", got)
	}
	if WithMemo(ctx) != ctx {
		t.Error("WithMemo on a memoised context should return it unchanged")
	}
}

func TestHelpersAndNoOp(t *testing.T) {
	ctx := ctxWith(map[string]any{"sub": "u", "tid": "t1", "sa": true})
	if tid, ok := TenantID(ctx); !ok || tid != "t1" {
		t.Errorf("TenantID() = %q, %v", tid, ok)
	}
	if _, ok := TenantID(context.Background()); ok {
		t.Error("TenantID() without claims should be absent")
	}
	if !IsSuperAdmin(ctx) || IsSuperAdmin(context.Background()) {
		t.Error("IsSuperAdmin mismatch")
	}
	n := NewNoOp()
	if !n.Can(context.Background(), "x:y") || n.Require(context.Background(), "x:y") != nil ||
		n.RequireIn(context.Background(), Scope{}, "x:y") != nil || n.Permissions(context.Background())[0] != "*" {
		t.Error("NoOp should allow everything")
	}
}

// sameCode reports whether err carries the errorz code of sentinel, which is
// what selects the HTTP status.
func sameCode(err, sentinel error) bool {
	var e, want *errorz.Error
	if !errors.As(err, &e) {
		return false
	}
	switch {
	case errors.Is(sentinel, errorz.ErrForbidden):
		want = errorz.Forbidden()
	case errors.Is(sentinel, errorz.ErrInternal):
		want = errorz.Internal()
	default:
		return errors.Is(err, sentinel)
	}
	return e.Code == want.Code
}
