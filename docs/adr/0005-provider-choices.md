# ADR 0005: Default technical providers, pending owner-supplied credentials

**Status:** Accepted (technical defaults only — see Open items)

## Context

The brief (§18) lists a long set of external providers the owner must supply (OAuth client IDs, FCM/APNs credentials, a maps provider, a transactional email provider, a licensed music provider or a decision to defer, billing accounts, AWS accounts). Its operating contract (§1.11) says: don't invent these values, but do all independent adapter/config/test work and hide the feature until configured, and don't repeatedly stall on the same missing credential while other work remains.

Several of these choices, though, have a technical dimension that doesn't require the owner's credentials to decide — only their product/vendor choice remains open. Deciding the technical shape now means later milestones aren't re-litigating architecture while only waiting on an API key.

## Decision

| Concern | Technical default (decided now) | Still open (needs owner input) |
| --- | --- | --- |
| Infrastructure as code | Terraform (the brief's own stated default, §12 M8) | AWS account, region, budget |
| Search | PostgreSQL `pg_trgm` (already an available extension per brief §6) for MVP scale; OpenSearch as a rebuildable projection once scale requires it (brief §11) | — (this one's fully decided; OpenSearch is additive later, not a fork in the road) |
| Push notifications | FCM (Android + cross-platform) and APNs (iOS) — not really a choice, these are the only viable providers for their platforms | Firebase project, APNs credentials |
| Native billing | Apple/Google in-app purchase for native digital goods — mandated by store policy for this content type, not a real choice | Store billing accounts; whether a separate web billing provider is also needed |
| OAuth | Google Sign-In and Sign in with Apple, matching the brief's explicit scope (§2) | Google Cloud project/client IDs, Apple Developer team/keys |
| Maps/places | Left fully open — no default, since brief lists no preferred vendor and pricing/coverage varies meaningfully by provider | Vendor choice itself |
| Transactional email | Left fully open | Vendor choice + sending domain |
| Analytics/crash reporting | Left fully open, but built behind an interface (see Consequences) so the choice is cheap to make later | Vendor choice |
| Licensed story music | Deferred — story music ships without a licensed track catalog until the owner decides to pursue one; the overlay/text/reaction features do not depend on it | Whether to pursue this at all |

## Consequences

- Every provider-dependent module (maps, email, push, analytics, billing beyond native IAP) is built behind a small Go interface (backend) or an abstract client (Flutter), so swapping the concrete vendor later touches one adapter, not call sites throughout the codebase.
- Any endpoint or UI element depending on an unconfigured provider stays hidden/disabled per brief §1.6 — this ADR doesn't change that rule, it just fixes the shape of the eventual adapter so the "hide until configured" behavior has a real feature flag to key off, not an ad hoc check.
- When the owner supplies a missing credential, the work is: fill in config, remove the feature flag, run the already-written adapter tests against the real sandbox — not redesign the integration.
- This ADR does not authorize creating any cloud resource, purchasing any service, or registering any developer account — those remain blocked on explicit owner authorization per brief §1.10, independent of the technical defaults recorded here.
