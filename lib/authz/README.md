# Authz Package

Permission checks for HTTP services: answers "may this caller do X?" from the claims `lib/auth` put on the request
context.

## Overview

`auth` proves who the caller is; `authz` decides what they may do. An `Authorizer` resolves the caller's effective
permissions from one of three sources and matches them against the permission a route or service needs. It is
the check behind `httpkit/middleware.RequirePermission`, and the tenant helpers (`TenantID`, `IsSuperAdmin`) feed
repository scoping in services built on the SDK.

## Features

- **Three sources**: `token` (a `perms` claim), `config` (a static role → permissions map), `identity` (a role map
  fetched from the identity service per tenant and cached for `cache_ttl`).
- **Wildcard grants**: `*` and `prefix:*` in grants; checks are always exact keys.
- **Super admin**: the `sa` claim allows everything.
- **Stale-while-error**: the identity source keeps serving the last role map while identity is down, for up to
  10 × `cache_ttl`, then fails with 503.
- **Per-request memo**: with `WithMemo(ctx)` (installed by the middleware) effective permissions resolve once per
  request.
- **Resource-scoped grants**: `RequireIn(ctx, scope, perm)` with a `ScopeResolver` for "a role on one thing".
- **NoOp**: `NewNoOp()` allows everything, for tests and local development.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/lib/authz
```

### Basic usage

```go
az, err := authz.New(&authz.Config{
    Source: authz.SourceConfig,
    Roles: map[string][]string{
        "admin":  {"*"},
        "member": {"notes:read", "notes:write"},
    },
})
if err != nil {
    return err
}

// Inside a handler or service, after middleware.Auth stored the claims:
if err := az.Require(ctx, "notes:write"); err != nil {
    return nil, err // 401 without claims, 403 without the permission
}
```

### Config (YAML)

```yaml
authz:
  source: identity                  # token | config | identity
  identity_url: http://identity:8080
  cache_ttl: 60s
  roles: {}                         # used by source config (and scoped roles with source token)
  perms_claim: perms
  tenant_claim: tid
  super_claim: sa
```

### Resource-scoped grants

```go
type staffResolver struct{ repo StaffRepository }

func (r staffResolver) RolesIn(ctx context.Context, s authz.Scope, userID string) ([]string, error) {
    return r.repo.RolesOn(ctx, s.ID, userID) // e.g. ["staff"] on this event
}

az, _ := authz.New(cfg, authz.WithScopeResolver(staffResolver{repo}))
// Load the event under the tenant scope first, then:
err := az.RequireIn(ctx, authz.Scope{Type: "event", ID: event.ID}, "guests:checkin")
```

Tenant-wide permissions are checked first; the resolver runs only when they miss. A resolver error is a 500.

## Options

| Option | Description |
|--------|-------------|
| `WithHTTPClient(c)` | HTTP client for the identity source. Default: 5s timeout. |
| `WithServiceToken(fn)` | Bearer token sent to identity's internal endpoint. |
| `WithScopeResolver(r)` | Enables scoped roles in `RequireIn`. |
| `WithLogger(l)` | Logs when a stale role map is served. Nil-safe. |
| `WithClock(now)` | Time source for cache expiry (tests). |

## Matching rules

- No claims → 401. Claims present, permission missing → 403.
- `*` matches everything; `notes:*` matches `notes:read` and `notes:read:any`; otherwise exact.
- `notes:read:any` does not imply `notes:read`; grant both when needed.
- A check that is empty or contains `*` never matches.

## Limitations

- **Role edits lag**: the identity source sees role edits within `cache_ttl`; the token source only at the next
  token refresh.
- **TenantID/IsSuperAdmin read the default claim names** (`tid`, `sa`). An Authorizer configured with other names
  uses them internally, but those helpers do not.
- **Scoped roles with the token source** are mapped through `roles` in config, since a token carries no role map.

## See also

- [auth](../auth/README.md) – validates tokens and stores the claims `authz` reads.
- [httpkit](../httpkit/README.md) – `middleware.RequirePermission` / `RequireAny` wrap `Require`.
- [errorz](../errorz/README.md) – denials are `errorz.Unauthorized()` / `errorz.Forbidden()` errors.
