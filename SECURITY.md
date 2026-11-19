# Security

go-sdk is the authentication, permission, encryption and HTTP layer of every service built on it, so a flaw here
affects all of them. This page covers how to report a vulnerability, what the security packages guarantee, and
the rules for using them safely.

## Reporting a vulnerability

**Do not open a public issue, PR or discussion for a security problem.**

Report it privately through GitHub: open the repository's **Security** tab and choose **Report a vulnerability**.
Include:

- the package and function, and what an attacker can do with the flaw;
- a test or code snippet that reproduces it;
- the version (tag or commit) you tested.

You can expect an acknowledgement within 7 days. Once the issue is confirmed, the fix is released as a new tag, and a
GitHub security advisory credits you unless you'd rather stay anonymous. Please give us a reasonable time to ship the
fix before you disclose it.

**In scope:** the packages under `lib/` and the `mocks` module.
**Out of scope:** misuse the rules below warn against (for example, a validator built without an expected issuer),
and vulnerabilities in third-party dependencies. Report those upstream. `govulncheck` in `make check` flags the
known ones.

## Supported versions

Only the latest `v0.x` tag gets fixes. The API is pre-1.0, so upgrade to the newest minor version to get a fix.

## What the packages guarantee

| Package | Guarantee |
|---|---|
| `auth` | A JWT validator accepts only the algorithm it was built for (`HS256` or `RS256`), so a token can't switch to `none` or to HMAC with a public key. Every token must carry `exp`. A JWKS key set refetches on an unknown `kid` at most once every 30 seconds |
| `authz` | No claims means 401, and a missing permission means 403. The `identity` source serves a stale role map for at most 10 × `cache_ttl` while identity is unreachable, then fails with 503 instead of granting. Wildcards apply to grants, never to the permission being checked |
| `crypto` | AES-256-GCM with a random nonce for every value. The encryption key must decode to exactly 32 bytes, and the blind-index key to at least 16 |
| `mailer` | Every message is validated before sending: required fields, address syntax (`net/mail`), and CR or LF in the subject (header injection) are refused |
| `httpkit` | An error response never carries an error's own text. A plain error, an `errorz` error without a message, and a recovered panic all answer `internal server error`. Only an `errorz` message reaches the client |
| `ratelimit` | The Redis backend counts with an atomic Lua script, so concurrent pods can't double-spend a budget |
| `logger`, all | Libraries return errors and never log on their own, so they never write secrets you didn't log yourself |

## Using the packages safely

These are the consumer's job. The packages allow the unsafe setting because some tests or local setups need it.

- **`auth`: always set the expected issuer and audience.** `WithExpectedIssuer` and `WithExpectedAudience` (or
  `issuer` and `audience` in config) are optional, and an empty value is **not checked**. Without them, a token
  issued for another service is accepted.
- **`auth`: use RS256 with JWKS across services.** HS256 shares one secret between the issuer and every validator,
  so any validator can mint tokens. Keep HS256 to a single process.
- **`auth`: keep `Policy` default-protected.** List public routes explicitly instead of listing protected ones.
- **`authz`: never use `NewNoOp()` outside tests and local development.** It allows every check.
- **`authz`: scope data by the token's tenant.** Use `authz.TenantID(ctx)` in every query on tenant-owned rows, never
  a tenant ID from the request.
- **`crypto`: keep the two keys distinct, random and backed up.** Generate them with `openssl rand -base64 32`, keep
  them in a secret store, and never derive one from the other. Losing them makes the data unreadable. A blind index
  is deterministic, so it reveals which rows share a value. Use it only for exact-match lookups on high-entropy
  values like emails.
- **`ratelimit`: decide the failure mode.** `httpkit/middleware.RateLimit` fails open when the backend errors. Add
  `WithFailClosed()` where letting requests through is worse than refusing them, and pair a fail-open limit with a
  per-account lockout for sign-in.
- **`ratelimit`: use the Redis backend when more than one instance runs.** The in-memory backend counts per process.
- **`httpkit`: return `errorz` errors with a message from handlers.** Anything else reaches the client as
  `internal server error`. Wrap causes with `errorz.Wrap(err).WithCode(...).WithMessage("...")`, and keep the
  message free of internal detail.
- **`httpkit/middleware.Logging`: leave `LogRequestBody` and `LogResponseBody` off** for any route that carries
  passwords, tokens or personal data.
- **`tracer/tracesql`: bind values as arguments, never format them into the SQL.** Spans record the statement
  text (`db.query.text`) but not arguments, so a value inlined into the query string reaches the trace backend.
- **`tracer.Transport`: name spans by route pattern.** A `WithSpanName` built from the raw path puts ids (and
  anything else in the path) into span names.
- **`sqlkit`: set `ssl_mode` to `require` or stricter** when the database is on another host.
- **`config`: keep secrets in the environment, not in committed YAML.** Reference them as `${VAR}`.

## Known limitations

- **No key-rotation helper in `crypto`.** Rotating the encryption key means re-encrypting the data yourself.
- **No token revocation in `auth`.** A valid JWT stays valid until `exp`. Keep access tokens short-lived.
- **Pre-1.0 API.** Security fixes may come with breaking changes in a minor version.
