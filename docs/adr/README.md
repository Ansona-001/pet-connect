# Architecture Decision Records

Each ADR here resolves an open question the completion brief flagged before letting implementation proceed (brief §12 Milestone 0: "Add ADRs for identity attribution, location privacy, media access, auth sessions, provider choices, and migration strategy"). They're written against concrete findings from the Day 1–2 codebase audit, not hypothetically — each cites the specific file/line that motivated the decision.

| ADR | Decision |
| --- | --- |
| [0001](0001-identity-attribution.md) | Likes/comments/follows carry both `user_id` (authorization) and optional `actor_pet_id` (display identity). |
| [0002](0002-location-privacy.md) | Exact coordinates return only to a resource's own owner; every other viewer gets distance/city-level data. |
| [0003](0003-media-access.md) | Media splits into a public tier (today's behavior) and a private tier (signed URLs / authorized proxy) — no private-media feature ships before the private tier exists. |
| [0004](0004-auth-sessions.md) | Remove the unused `JWT_REFRESH_SECRET`; add a Redis-backed invalidation timestamp so logout-all/security events revoke access tokens and sockets promptly instead of waiting for natural expiry. |
| [0005](0005-provider-choices.md) | Technical defaults recorded now (Terraform, pg_trgm→OpenSearch, FCM/APNs, native IAP) so only vendor accounts/credentials remain open, each behind a swappable adapter. |
| [0006](0006-migration-strategy.md) | `000001_init.sql` may be split into schema + seed once, since nothing has deployed it yet; every migration after that is additive-only. |

An ADR here is binding until superseded by a new one — don't silently work around a decision recorded above; if it turns out wrong, write a new ADR that supersedes it and say why.
