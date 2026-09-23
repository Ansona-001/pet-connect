-- Adds durable, database-tracked media objects and an ordered post/media
-- join table, replacing posts.media_url/media_type's one-image-per-post
-- ceiling for any post that wants a carousel (brief Milestone 3: "Ordered
-- media schema"). Today an upload (media.upload's POST /media/uploads)
-- writes straight to S3/MinIO and returns a path the client passes into
-- POST /posts' media_url — no row anywhere in Postgres ever represents
-- "this file was uploaded," which is exactly what the next few days'
-- work needs a stable id to attach to: upload validation results,
-- re-encode/EXIF-strip outcomes, and (much later in this milestone) the
-- transcode worker and orphaned-media cleanup job both need to know what
-- media exists at all, not just what a post happens to reference.
--
-- posts.media_url/media_type are left untouched — additive-only per ADR
-- 0006. The "Carousel post API" day is what teaches post create/edit/
-- read to prefer post_media when a post has rows there, falling back to
-- the legacy single-media columns for every post created before this
-- migration existed. This migration only adds the two tables.
CREATE TABLE media (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  media_type text NOT NULL CHECK (media_type IN ('image', 'video', 'audio')),
  storage_path text NOT NULL,
  content_type text NOT NULL,
  byte_size bigint NOT NULL,
  width int,
  height int,
  duration_ms int,
  created_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE INDEX media_owner_idx ON media(owner_user_id, created_at DESC) WHERE deleted_at IS NULL;

-- post_media orders a post's media items (position 0, 1, 2, ...) for
-- carousel rendering. ON DELETE RESTRICT on media_id is deliberate: a
-- media row still referenced by a live post cannot be deleted out from
-- under it — the later orphaned-media cleanup job only ever targets
-- media with zero post_media rows, so this constraint is a backstop
-- against that job (or anything else) ever getting that wrong.
CREATE TABLE post_media (
  post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  media_id uuid NOT NULL REFERENCES media(id) ON DELETE RESTRICT,
  position int NOT NULL CHECK (position >= 0),
  PRIMARY KEY (post_id, media_id),
  UNIQUE (post_id, position)
);
