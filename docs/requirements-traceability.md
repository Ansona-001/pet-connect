# Requirements Traceability Matrix

Seeded from the feature truth table in `PETCONNECT_AI_AGENT_COMPLETION_BRIEF.md` §9, then cross-checked against the actual code in this repository (not assumed) as of Day 2. "Verified" notes below are things confirmed directly against `server/internal/**` and `lib/**` during this pass; anything not marked verified carries the brief's original claim forward unchanged. Update the **Status** and **Evidence** columns as each milestone slice lands — this file is the source of truth the brief's operating contract (§1.3) requires.

Legend — **Status**: `Working core` / `Partial` / `Demo only` / `Schema only` / `Missing`. `Δ` marks a status this pass changed from the brief's original snapshot.

## Identity, auth & account

| Requirement | Status | Schema | API | Flutter client | Tests | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| Email registration/login | Working core | `users` (000001_init.sql) | `auth.RegisterRoutes` → `POST /auth/register`, `/auth/login` (`server/internal/modules/auth/auth.go:51`) | `lib/features/auth/` | `internal/modules/auth/*_test.go` | No email verification, reset, or abuse controls yet (M1). |
| JWT/refresh/logout | Working core | `refresh_tokens` | `POST /auth/refresh`, `/auth/logout` | `lib/core` Dio interceptor | `internal/modules/auth/*_test.go` | Verified: access JWTs and sockets aren't revoked on logout — no blacklist/short-TTL check exists in `httpx.Authenticate`. |
| Google/Apple sign-in | Missing | — | — | UI explains credentials required | — | Blocked on OAuth credentials (brief §18). |
| Onboarding | Partial | `users`, `pets` | `PATCH /me`, `POST /me/pets` | `lib/features/onboarding/` | none found | Owner patch and pet creation are separate calls, not atomic. |
| Owner profile | Partial | `users.location` (lat/lng) | `GET/PATCH /me` | partial | `internal/modules/account/*_test.go` | **Verified new finding:** `GET /me` returns raw `location.latitude/longitude` with no rounding (`account.go:38-41`) — a direct instance of brief defect #7. No public owner profile endpoint exists yet. |
| Multi-pet profiles | Partial | `pets` | `pets.RegisterRoutes` (`pets.go:52-57`) — full CRUD | partial | `internal/modules/pets/*_test.go` | **Verified new finding:** `pets.location` also returns raw lat/lng (`pets.go:60-63`), same exposure as `/me`. |
| Blocks/mutes/reports | Partial Δ | `blocks`, `mutes`, `reports` (000004_safety_primitives.sql) | `safety.RegisterRoutes` → `POST/GET /blocks`, `DELETE /blocks/:userId`, `POST/GET /mutes`, `DELETE /mutes/:userId`, `POST /reports` (`server/internal/modules/safety/safety.go`) | none yet | none | Live-verified Day 19: feed/discovery/matching-candidates exclude blocked users both directions; feed also excludes muted users (mute doesn't affect discovery/matching); blocking sets any active `matches.status='blocked'`, which the pre-existing chat module already treats as terminal (hides from `GET /chats`, rejects `SendMessage`) — no chat module changes needed. Comments/stories and a Flutter UI are not yet wired; the M2 "Visibility enforcement" and M4 "Unmatch, block, report" days own the remaining broader audit and UI. |
| Sessions/devices | Missing | none | none | none | — | `JWT_REFRESH_SECRET` is loaded/validated but unused (opaque refresh tokens) — dead config per brief §6. |

## Social graph & content

| Requirement | Status | Schema | API | Flutter client | Tests | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| Feed | Partial | `posts`, `post_likes`, `post_saves` | `GET /feed`, `PUT/DELETE .../like`, `.../save` (`social/routes.go:13-20`) | `lib/features/social` | none in `social` package (0 test files) | Cursor pagination exists (`timeCursor`, `models.go:97`) but feed ranking/infinite scroll on client is unverified this pass. |
| Video/carousel posts | Missing | `Post` has one `media_url`/`media_type` field, no ordered media table | same as above | — | — | **Verified:** `social/models.go:18-36` confirms single-media-per-post; no `post_media` table exists in `000001_init.sql`. |
| Stories | Partial | `stories` | `GET/POST /stories` | simple viewer | none | `Story.TextOverlay` is a stored `map[string]any` — overlay data persists, but no views/reactions/replies tables exist. |
| Reels | Partial/API-only | reuses `posts` table | `GET/POST /reels` (`social/routes.go:25-26`) | reuses feed UI | none | Confirmed reels are not a distinct schema — same `Post`/`Comment` types, just filtered. |
| Follows | Schema only | `follows` table exists in `000001_init.sql` | **none** — no route registers `/follows/*` anywhere in `cmd/api/main.go` | local-only in reel screen | — | Verified: table exists, zero backend routes, zero server-side enforcement of follow-gated visibility. |
| Comments | Partial | `comments` | `GET/POST /posts/:postId/comments` | — | none | No edit/delete endpoints exist yet (create + list only). |
| Hashtags/mentions/shares | Missing | none | none | — | — | No parsing, no share endpoint, no hashtag table. |

## Discovery & matching

| Requirement | Status | Schema | API | Flutter client | Tests | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| Nearby discovery | Partial/API-only | PostGIS query in `discovery` module | `GET /discovery/pets` (`discovery/routes.go:11`) | `discover_screen.dart` uses `MockSocialData` | 0 test files in `discovery` package | **Verified:** `Candidate.DistanceKM` (`discovery/handler.go:34`) is already privacy-safe (rounded distance, not coordinates) — better than the brief's general warning suggested for this specific endpoint. |
| Pet swipe/match | Working core | `pet_swipes`, `matches`, auto `chats` row | `POST /pets/:petId/swipes`, `GET /matches` (`matching/handler.go:23-25`) | match screen | `internal/modules/matching/*_test.go` | **Verified:** swipe already carries a `ClientRequestID` for idempotency (`matching/types.go:63`) — concurrency handling is more built-out than the truth table implies. |
| Playdates | Missing | none | none | — | — | No schema at all yet. |
| Search (pet/owner/hashtag) | Missing | none | none | — | — | Confirmed no `/search` route and no OpenSearch/trigram wiring in `cmd/api/main.go`. |

## Chat, notifications, push

| Requirement | Status | Schema | API | Flutter client | Tests | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| Text chat | Working core | `chats`, `chat_members`, `messages` | `chat.Register` (`chat/handler.go:23-26`) | chat screen | `internal/modules/chat/*_test.go` | **Verified:** messages already carry `ClientMessageID` for idempotent send (`chat/types.go:24,57`) and `MessageType`/`MediaURL` fields exist server-side — schema is ahead of the Flutter client here. |
| Image/video/audio/location chat | Backend partial/client missing | `messages.message_type`, `media_url` columns exist | send accepts any `message_type` string (unvalidated against an enum in the handler I read) | placeholders | — | Backend accepts the fields; nothing enforces which `message_type` values are valid yet. |
| Presence/typing/receipts | Partial | — | `PUT /chats/:chatId/read`; typing/read socket events referenced in brief §6 | not wired | — | Not independently re-verified this pass; carried from brief. |
| In-app notifications | Backend only | `notifications` | `GET /notifications`, `PUT .../read-all`, `PUT .../:id/read` | placeholder/fake | none | **Verified defect, exactly as brief describes:** `listResponse` (`notifications.go:53-57`) has `HasMore bool` but **no `NextCursor` field at all** — `has_more: true` is structurally impossible to page through today. |
| Push notifications | Missing | none | none | none | — | No device/token table, no FCM/APNs wiring. |

## Communities, events, adoption

| Requirement | Status | Schema | API | Flutter client | Tests | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| Communities | Backend partial/client mock | `communities`, `community_members` | list/create/join/leave only (`community/routes.go:11-14`) | mock content | 0 test files | No details/roles/discussions/moderation endpoints exist. |
| Events | Backend partial/client mock | `events`, `event_rsvps` | list/create/RSVP only (`events/routes.go:11-14`) | mock state | 0 test files | **Verified new finding:** `Event.Latitude`/`Longitude` (`events/handler.go:33-34`) are returned as raw exact coordinates with no rounding — same class of leak as `/me` and `/pets/:petId`, and arguably higher-risk since events imply a real meeting point. Flag for the M4 location-privacy audit. |
| Adoption | Backend partial/client mock | `adoption_listings`, `adoption_saves` | list/create/save only (`adoption/routes.go:11-14`) | mock state | 0 test files | No organizations, verification, applications, or status history schema yet. |

## Platform, security, ops

| Requirement | Status | Notes |
| --- | --- | --- |
| Media upload/access | Missing hardening | **Verified:** `media.Handler.upload` (`media/handler.go:34-71`) validates only the client-supplied `Content-Type` header against an allowlist — no magic-byte sniffing, no re-encoding, no scanning. **Verified:** `GET /media/uploads/*` (`media/handler.go:73-85`) is mounted on the **public**, unauthenticated router (`h.Register(protected, public)` in `cmd/api/main.go:151` — `download` goes on `public`) — anyone with a URL can fetch any uploaded object today, private or not. This is a concrete instance of brief defect #6/§15 "never authorize private media merely because someone knows a URL," not just a theoretical risk. |
| Rate limiting/abuse controls | Missing | No `limiter` middleware registered in `cmd/api/main.go`'s middleware stack (only `requestid`, `recover`, `helmet`, `cors`, `compress`, `logger`). |
| Premium/entitlements/billing | Missing | No schema, no routes. |
| Analytics/telemetry | Missing | No event emission found anywhere in `lib/` or `server/`. |
| Production infra | Missing | `compose.yaml` (local only), no `infra/`, no CI beyond what M0 Day 4 adds. |

## Backend test coverage (verified, Day 1 finding carried here for visibility)

Packages **with** `_test.go` files: `account`, `auth`, `chat`, `matching`, `notifications`, `pets`, `realtime`.
Packages with **zero** test files: `adoption`, `community`, `discovery`, `events`, `media`, `social`. `go test ./...` reports `ok` for these too — they simply have nothing to fail.

## Open items this matrix will track going forward

- Every day's slice in the build planner should update the relevant row's Status/Evidence/Tests columns before that day is marked done.
- Three raw-coordinate exposures are now confirmed (not hypothetical): `GET /me`, `GET /pets/:petId`, and `GET/POST /events`. These need the location-privacy pass (ADR 0002, M1/M4 work) before any of those endpoints are called by production-facing UI beyond the current owner-only screens.
- The unauthenticated `GET /media/uploads/*` route needs to move behind an authorization check (or signed URLs) before any private media type is introduced — currently low-risk only because all uploads are effectively public today, which is itself the gap to close.
