# Portfolio security hardening plan

Based on [the 2026-10-08 audit](../security/AUDIT-2026-10-08.md), baseline `91b6136`, public origin `https://imaginarymakings.dev`. This file is a proposed implementation contract; the audit did not implement or deploy these changes.

## 1. Bound public login work — highest priority

Addresses F01/F02. Files: `internal/handlers/admin.go`, `internal/auth/password.go`, `internal/handlers/pages.go` for shared dependencies, SQL/migrations/generated queries if using a database-backed reservation.

- Install a small login body cap before any form access; reject multipart/unsupported bodies, malformed forms, oversized fields and chunked overflows explicitly.
- Reserve per-client attempt budgets atomically before Argon2; add bounded global hashing concurrency and a finite overload response. Do not hold a SQLite write transaction during hashing.
- Handle limiter persistence errors, define refill/backoff semantics, and preserve generic login failures. Do not let distributed traffic create unbounded limiter keys/queues.
- Preserve admin media's separate size budget and legitimate login behavior.

Acceptance: deterministic concurrent-budget tests, maximum hashing-concurrency tests, DB failure handling, and body-reader tests proving oversized/chunked/multipart login inputs are rejected before expensive work. Convert relevant opt-in audit reproductions into normal negative regression tests.

## 2. Define production origin and trusted proxy boundaries

Addresses F03/F09; can proceed with phase 1, but final limiter integration depends on correct client identity. Files: `cmd/server/main.go`, handlers, deployment config/docs.

- Validate canonical `PUBLIC_BASE_URL=https://imaginarymakings.dev`; use it for RSS/OAuth and enforce expected production hosts/schemes.
- Make proxy trust explicit, with tests for direct peers, trusted peers, malformed headers and multi-hop chains. Do not trust all private addresses by default.
- Ensure Caddy validates Cloudflare client identity and emits one app-trusted value; enforce loopback Go binding and origin firewall rules.
- Require secure production cookies; keep explicit local-development behavior.

Acceptance: direct forged headers cannot evade rate limits; distinct legitimate users remain distinct; unexpected Host is rejected; canonical HTTPS RSS/callback links; actual origin listener/firewall check and live cookie/header verification.

## 3. Revoke sessions on credential changes

Addresses F04; independent of phases 1–2. Files: auth/session code, database migration and queries, configuration docs.

- Associate each session with a credential generation; reject old generations after rotation/restart. Invalidate legacy sessions safely during rollout.
- Implement revoke-all and fail-safe logout behavior; expire/prune old sessions and OAuth requests.
- Document emergency revocation and rotation without logging secrets.

Acceptance: a previously valid session fails immediately after rotation, current login succeeds, all mutation routes reject revoked cookies, and logout DB errors do not falsely report successful revocation.

## 4. Browser policy, cache rules and public errors

Addresses F05/F08. Files: route middleware/server setup, templates/static scripts, Caddy/Cloudflare settings.

- Emit no-store on every admin response and enforce CDN bypass on `/admin` and `/admin/*`; bypass health caching.
- Add frame denial, nosniff, referrer policy and an HSTS policy appropriate for the domain.
- Externalize inline handlers/scripts or introduce a tested nonce/hash design before enforcing CSP; keep HTMX preview and photo modal behavior working.
- Return generic public 500 errors and log internal details server-side with correlation IDs.

Acceptance: headers on 2xx/3xx/4xx/5xx, two independent admin sessions never sharing cached HTML, browser login/edit/preview/upload/logout/gallery flows under CSP, no private error details for multiple Accept formats. Update generated templ output with source changes.

## 5. Patch and continuously assess dependencies

Addresses F06. Files: `go.mod`, `go.sum`, `Dockerfile`, `.github/workflows/ci.yml`, tool pins if required.

- Select a supported patched Go version (at least 1.26.6 for the identified standard-library advisories) and x/text at least v0.41.0; align other x/* modules and codegen tools as compatibility requires.
- Add pinned govulncheck and scheduled scans. Document narrowly justified findings that are outside runtime configuration; do not disable broad classes of advisories.
- Pin reviewed action SHAs, constrain workflow permissions and validate downloaded tools/assets.

Acceptance: build/test/vet on Linux ARM64, source and actual release-binary scan, known generation-drift checks, and verified production build metadata after rollout. Review runtime reachability separately from scanner counts.

## 6. Bound public reads and validate host operations

Addresses F07 and deployment backlog; does not block initial login fixes. Files: Flickr store/queries, public handlers, deployment docs.

- SQL-level photo pagination and direct/adjacent/random selection; bounded journal/feed behavior and snapshot-aware caching.
- Exact home route/404s, optional Swagger removal, explicit cache invalidation on publication/privacy changes.
- Review active systemd limits, deployment staging trust, runner isolation, branch protection, secret permissions, monitoring, and restore procedures.
- Consider MFA access-gating for admin, preserving Flickr callback behavior.

Acceptance: disposable large-catalog work bounds and functional pagination tests; no live load testing; read-only host configuration evidence; disposable backup restore; production read-only smoke checks after approved rollout.

## Completion criteria

Every finding is either fixed with regression evidence or explicitly accepted with its deployment conditions documented. Passing current audit reproductions is not completion. Preserve existing content/databases throughout; use disposable stores for tests. Deployment must verify the new artifact and live protections separately from local test results.
