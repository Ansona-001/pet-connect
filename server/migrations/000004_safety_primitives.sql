-- Adds the safety primitives the brief calls out as entirely missing
-- (docs/requirements-traceability.md: "No table, no endpoint. Ungated in
-- every read path today."): blocks, mutes, and reports. Additive-only per
-- ADR 0006.
--
-- All three are deliberately user-scoped, not pet-scoped, matching ADR
-- 0001's reasoning that user_id is the durable authorization/moderation
-- anchor while pets are just a display identity: blocking "Milo's owner"
-- should sever the whole relationship, not just one of their pets.

-- Blocking is directional in storage (who blocked whom, and when) but
-- enforced bidirectionally in every read path this migration's companion
-- code touches — a block should stop either party from seeing the other,
-- not just the direction the block button was pressed in.
CREATE TABLE blocks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  blocker_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  blocked_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (blocker_user_id, blocked_user_id),
  CHECK (blocker_user_id <> blocked_user_id)
);

CREATE INDEX blocks_blocker_idx ON blocks(blocker_user_id);
CREATE INDEX blocks_blocked_idx ON blocks(blocked_user_id);

-- Muting is deliberately much narrower than blocking: it only affects
-- what the muter's own feed shows them (see the feed query changes in
-- the social module), doesn't touch matching/discovery/chat at all, and
-- is never visible to the muted person. No CHECK/index pair is needed
-- beyond what blocks already has, so this mirrors that table's shape.
CREATE TABLE mutes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  muter_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  muted_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (muter_user_id, muted_user_id),
  CHECK (muter_user_id <> muted_user_id)
);

CREATE INDEX mutes_muter_idx ON mutes(muter_user_id);

-- Reports are polymorphic (a user can report a user, pet, post, comment,
-- story, or message) so subject_id deliberately has no foreign key —
-- there's no single table it could reference. reported_user_id is always
-- resolved and stored redundantly at write time (see safety.go) so a
-- moderation queue can filter/sort "reports about this person" without
-- joining out to five different tables per subject_type.
CREATE TABLE reports (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  reporter_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reported_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  subject_type text NOT NULL CHECK (subject_type IN ('user', 'pet', 'post', 'comment', 'story', 'message')),
  subject_id uuid NOT NULL,
  reason text NOT NULL CHECK (reason IN ('spam', 'harassment', 'inappropriate_content', 'fake_profile', 'animal_welfare', 'other')),
  details text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'reviewing', 'resolved', 'dismissed')),
  created_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz
);

CREATE INDEX reports_reported_user_idx ON reports(reported_user_id, created_at DESC);
-- Partial index sized for the moderation queue's actual access pattern
-- (open reports, oldest first) rather than the whole table.
CREATE INDEX reports_open_idx ON reports(created_at ASC) WHERE status = 'open';
