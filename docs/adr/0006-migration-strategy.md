# ADR 0006: Migration strategy and demo-seed separation

**Status:** Accepted

## Context

`server/migrations/` contains exactly one file, `000001_init.sql`, which both creates the full schema and inserts demo accounts/content (brief §8). Migrations are embedded and applied at API startup (`migrations/embed.go`).

Per Day 1's audit, this repository has never been deployed anywhere shared: there is no `infra/`, no staging/production environment, and no evidence of the API having run against any database other than the local Docker Compose stack. The brief's migration warning (§8) is written for the general case where that might not be true, and instructs checking before restructuring — this ADR records that check having been done, and its result: it is currently safe to restructure `000001_init.sql` directly, because nothing depends on its current shape yet.

## Decision

- **One-time exception, now:** split `000001_init.sql` into a clean schema-only migration plus a separate, explicitly non-production seed command, as Day 5 of the build plan. This is done as a direct edit to `000001_init.sql`, not a new migration on top of it, specifically because no shared environment has applied it yet. This exception does not repeat — see below.
- **From this point forward, all migrations are additive-only.** No migration after this split may drop or rename a column/table that existing code still reads; changes that would otherwise be destructive go through an expand/contract sequence (add the new shape, migrate readers/writers to it, only then remove the old shape in a later migration once nothing references it).
- **Seed data is never part of a numbered migration again.** The dev/test seed becomes a standalone command (e.g. `go run ./cmd/seed` or an equivalent), gated to refuse running when `APP_ENV` indicates staging/production, matching brief §8 item 4.
- **Concurrency:** add a Postgres advisory lock (`pg_advisory_lock`) around migration application at startup now, while the deployment topology is a single local API process, so the pattern already exists before Milestone 8 introduces multiple replicas. At that point, migrations move to a one-off pre-traffic-shift job (brief §12 M8) instead of running from every replica's startup path; the advisory lock is a safety net for that transition period, not a replacement for it.

## Consequences

- Day 5's seed-separation work directly implements this ADR's first two bullets.
- Every future migration PR must be reviewed against the additive-only rule; a migration that renames or drops a column in one step is a defect from this point on, not a style preference.
- Once any environment beyond local development exists (even a shared dev database), this ADR's one-time exception is closed permanently — from that moment, `000001_init.sql` itself becomes immutable like every migration after it, per the brief's general rule.
- The advisory-lock addition is a small, low-risk change appropriate for Milestone 0; the one-off migration job is explicitly Milestone 8 scope and not pulled forward.
