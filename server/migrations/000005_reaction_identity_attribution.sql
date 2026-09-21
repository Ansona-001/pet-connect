-- Applies ADR 0001's identity-attribution decision (docs/adr/0001-
-- identity-attribution.md) to likes and comments: store the durable,
-- non-spoofable user_id (already present on both tables) alongside an
-- optional actor_pet_id, so the UI can display "Milo liked this" instead
-- of always falling back to the owner's own name. Follows already
-- shipped (Milestone 2's "Follow/unfollow" day) without this column —
-- ADR 0001 covers it too, but that's its own, separate follow-up; this
-- migration is scoped to what today's slice actually touches.
--
-- Backfill: every existing row gets actor_pet_id = NULL (owner-
-- attributed) — ADR 0001 is explicit that "no data implies a false pet
-- attribution" rather than guessing which of an owner's pets should
-- retroactively take credit for a reaction made before this column
-- existed. NULL is also a new nullable column's own default, so no
-- UPDATE statement is needed to achieve this — the migration's entire
-- job is adding the column safely, additive-only per ADR 0006.
ALTER TABLE post_likes ADD COLUMN actor_pet_id uuid REFERENCES pets(id) ON DELETE SET NULL;
ALTER TABLE comments ADD COLUMN actor_pet_id uuid REFERENCES pets(id) ON DELETE SET NULL;
