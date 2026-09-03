# ADR 0001: Identity attribution for likes, comments, and follows

**Status:** Accepted

## Context

The brief (§3 Identity) establishes `user_id` as the authentication, billing, consent, and audit identity, while posts, stories, reels, matching, and playdates belong to a pet. It flags that likes/comments/follows are currently user-scoped and asks for a decision before the social graph (Milestone 2) expands.

Verified against the actual schema (`server/internal/modules/social/models.go`):

- `Comment.UserID` — attributed to the user, no pet reference.
- `Post.AuthorUserID` — same.
- `post_likes` (schema) — user-scoped, per brief §9.
- The `follows` table exists but has zero registered routes today, so it isn't yet attributed either way.

A user may own several pets (multi-pet is a core feature). When Milo's owner comments on another pet's post, is that comment "from the owner" or "from Milo"? The UI needs a pet identity to show (name + avatar), but authorization, moderation, and abuse tooling need a durable, non-spoofable user identity that survives a pet being deleted or transferred.

## Decision

Store **both** `user_id` and an optional `actor_pet_id` on likes, comments, and follows (this is the brief's own recommended model, and we're adopting it as-is rather than inventing an alternative):

- `user_id` is required and is the authorization/audit anchor — a user can never act through a pet they don't own (brief §3), enforced by checking `pets.owner_id = user_id` at write time regardless of what `actor_pet_id` claims.
- `actor_pet_id` is nullable. When set, the UI displays the pet's identity (name, avatar) instead of the owner's. When null, the action is owner-attributed (e.g. joining a community isn't naturally pet-scoped).
- Follows specifically: a follow relationship is `(follower_user_id, actor_pet_id?, followed_pet_id)` — you follow a *pet*, optionally as one of your own pets, so "Milo follows Coco" is representable distinctly from "the owner follows Coco" with no active pet context.

## Consequences

- Migration required: add nullable `actor_pet_id uuid references pets(id)` to `comments`, `post_likes`, and the new `follows` routes' backing rows, with a backfill (existing rows get `actor_pet_id = NULL`, i.e. owner-attributed — no data implies a false pet attribution).
- Every write handler for these three resources must validate `actor_pet_id` (if present) belongs to the authenticated `user_id` before insert — this is the concrete enforcement of "a user may never act through a pet they do not own."
- Read/serialization: API responses expose both fields; Flutter decides which identity to render (pet if present, else owner).
- This is scoped work for Milestone 2 ("Apply the identity-attribution ADR consistently to likes/comments/follows").
