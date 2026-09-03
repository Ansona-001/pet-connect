# ADR 0003: Media upload validation and access control

**Status:** Accepted

## Context

Verified against `server/internal/modules/media/handler.go`:

- `upload` (line 34) validates a file only by trusting the client-supplied `Content-Type` header against an allowlist. No magic-byte sniffing, no re-encoding, no EXIF stripping, no malware scan.
- `download` (line 73) is registered via `h.Register(protected, public)` with `download` mounted on the **public** router (`cmd/api/main.go:151`) — it has no authentication check at all. Anyone who obtains a storage key/URL can fetch the object, whether or not they're authorized to see it.

The brief (§15) requires upload signature sniffing, safe filenames, size/dimension/duration limits, malware scanning, image re-encoding, EXIF removal, and states plainly: "never authorize private media merely because someone knows a URL." It also lists private/signed access as a release gate wherever authorization can change (chat attachments, adoption documents, moderation evidence).

## Decision

Split media into two access tiers instead of one:

1. **Public media** — profile photos, pet photos, and any post/story/reel content once its visibility rules allow public viewing. Served from a CDN-cacheable path with long cache headers, exactly as `download` does today. This tier's risk is limited to "everything on it is meant to be visible to whoever has the link," which matches today's actual content.
2. **Private media** — chat attachments, adoption application documents, and anything else where authorization can change per-viewer. These require either signed, time-limited URLs generated per request, or a proxy endpoint that re-checks resource-level authorization (e.g. "is this requester a member of this chat?") on every fetch, not just at upload time.

Until tier 2 is implemented, **no private-media feature ships** — this is the practical reading of "never authorize private media merely because someone knows a URL." The current unauthenticated `download` route stays as the public-tier implementation; it is not a bug to fix today, but a boundary not to cross until tier 2 exists (chat media, adoption documents, and any moderation evidence are blocked on this ADR).

Upload hardening lands independently of the two-tier split, since it protects both tiers equally:

- Sniff actual file bytes (not just the header) to confirm the content type before accepting or serving a file.
- Re-encode images server-side (strips EXIF as a side effect, and neutralizes several classes of malformed-image exploit).
- Enforce size/dimension/duration limits server-side, not just client-side.
- Generate storage keys that never embed client-controlled filenames verbatim (already true today — `media/handler.go:59` uses a generated UUID, this stays).

## Consequences

- Milestone 3 ("media magic-byte validation, image re-encoding/EXIF removal") implements the sniffing/re-encoding work.
- Any milestone introducing a private-media use case (chat image/video/voice attachments in M5, adoption application documents in M6) must implement the signed-URL or authorized-proxy pattern as part of that slice — it cannot reuse the current public `download` route.
- `openapi/v1.yaml`'s note on `GET /media/uploads/{key}` stays accurate as documentation of the public tier; it should gain a sibling private-media path once tier 2 exists, not a retroactive rewrite of the existing one.
