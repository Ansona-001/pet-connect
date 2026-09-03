# ADR 0004: Auth session lifecycle and revocation

**Status:** Accepted

## Context

Verified against `server/internal/modules/auth/auth.go` and `internal/platform/httpx/httpx.go`:

- Refresh tokens are opaque, rotated on use, and reuse revokes the token family — this part already meets the brief's bar (§13 "Refresh rotation is atomic; reuse revokes the family").
- `JWT_REFRESH_SECRET` is loaded and validated at startup but never referenced anywhere else in the codebase — refresh tokens are random opaque values, not JWTs, so the secret has no purpose. This matches the brief's own observation (§6) and is "dead security configuration" per its instruction not to leave such things in place.
- `httpx.Authenticate` (`httpx.go:47`) parses and validates the JWT's signature and expiry only. It has no lookup against any revocation list. Logging out revokes the *refresh* token but does nothing to an already-issued *access* token or an open WebSocket connection — both remain valid until natural expiry/disconnect (brief defect #4, and explicitly required to close before Milestone 1's gate: "Revoked sessions lose REST and WebSocket access according to documented latency").

## Decision

**Dead config:** Remove `JWT_REFRESH_SECRET` rather than repurpose it. Opaque random tokens don't need a signing secret; inventing a use for it just to keep the variable would be worse than deleting it. This is a configuration-surface change, documented here so it isn't mistaken for an oversight later, and called out in the changelog when it lands.

**Revocation latency:** Add a lightweight, near-immediate revocation check rather than either (a) doing nothing, which fails the M1 gate, or (b) building full per-token blacklisting, which is disproportionate for a JWT-based access token scheme. Mechanism:

- Store a `sessions_invalidated_after` timestamp per user in Redis (or Postgres, whichever the sessions table lands in during Milestone 1's session/device work), set on logout-all, password change, and any security event.
- `httpx.Authenticate` gains one additional check: if the JWT's `iat` (issued-at) predates the user's `sessions_invalidated_after`, reject with the same `invalid_access_token` response it already returns for expired/malformed tokens. This is a single Redis GET per request — cheap, and the "documented latency" the brief asks for becomes "as fast as the next request after invalidation," which is effectively immediate.
- Plain single-session logout (not logout-all) can rely on short access-token TTLs plus refresh-token revocation, since the brief accepts *documented* latency, not necessarily zero latency, for the common case — full invalidation is reserved for logout-all and security events where the bar is higher.
- WebSocket connections: on a revocation event, publish to the realtime hub (`internal/realtime`) to force-close that user's open sockets, rather than waiting for their next ping/pong cycle to fail naturally.

## Consequences

- Requires a Redis read on every authenticated request — acceptable, since Redis is already in the hot path for realtime and rate limiting, and the value is small and cacheable.
- `internal/platform/httpx/httpx.go`'s `Authenticate` middleware needs a Redis client injected; currently it only depends on `authjwt.Manager`.
- Session/device listing and logout-all (Milestone 1 tasks) are the natural place to wire the write side of this (`sessions_invalidated_after` updates).
- Tests must cover: a token issued before invalidation is rejected after logout-all; a token issued after invalidation is still accepted; an open socket is closed on revocation.
