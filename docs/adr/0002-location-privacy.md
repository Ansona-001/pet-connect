# ADR 0002: Location privacy — who gets exact coordinates

**Status:** Accepted

## Context

The brief (§3 Location and privacy) is explicit: exact home/pet coordinates are sensitive, must never be returned to unrelated users, logged, or put in analytics; public APIs should expose city/area, approximate map coordinates, and/or rounded distance.

Day 2's traceability pass verified this is **not yet true** in three places:

- `GET /me` returns `location.latitude/longitude` unrounded (`server/internal/modules/account/account.go:38-41`).
- `GET /pets/:petId` returns the same shape unrounded (`server/internal/modules/pets/pets.go:60-63`) — and this endpoint is reachable for *any* pet, not just the caller's own.
- `GET/POST /v1/events` returns exact `latitude`/`longitude` (`server/internal/modules/events/handler.go:33-34`), visible to every RSVP'd or browsing user.

By contrast, `GET /v1/discovery/pets` already does this correctly — it returns `distance_km`, never coordinates (`discovery/handler.go:34`). That endpoint is the model to generalize from, not the exception.

## Decision

Adopt a single rule applied consistently everywhere a location is serialized: **exact coordinates are returned only to the resource's own owner; every other viewer gets a privacy-reduced representation.**

Concretely:

- `GET /me`: exact coordinates only in the response to the authenticated owner themself (already true today by construction — but must stay true once a public owner-profile endpoint is added in Milestone 2, which must *not* reuse this serializer as-is).
- `GET /pets/:petId`: exact coordinates only when `requester_user_id == pet.owner_id`. All other requesters (including public/follower views) get either omitted location, a city/area string, or a coarsened point (coordinates rounded to ~2 decimal places, ≈1.1km, is the floor of acceptable coarsening — city/area text is preferred where available).
- Events: an event's `location_name` (human-readable) stays public; exact `latitude`/`longitude` are released only after RSVP confirms attendance, or the event is explicitly marked public-exact by its creator. Until that policy is built, default to withholding exact coordinates from anyone except the creator and confirmed attendees.
- Discovery/matching keep using `distance_km` — no change needed, this is already the correct pattern.
- Logging and analytics: never pass a raw `location` struct into `slog` fields or analytics event payloads — pass city/area or nothing. This closes brief defect #7 ("audit every serializer, log, event, and analytics payload").

## Consequences

- A shared serialization helper (e.g. `location.ForViewer(loc, isOwner bool)`) should replace the current pattern of handlers marshaling the raw struct directly, so the rule can't be silently skipped in a new handler.
- Requires touching `account`, `pets`, and `events` handlers, plus any future endpoint that serializes a location.
- Tests must cover the boundary explicitly: owner sees exact coordinates, non-owner does not, for each of the three endpoints above.
- Tracked as the M1/M4 "location-privacy audit" line item in `docs/requirements-traceability.md`.
