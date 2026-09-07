CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email citext NOT NULL UNIQUE,
  password_hash text NOT NULL,
  name text NOT NULL,
  bio text NOT NULL DEFAULT '',
  city text NOT NULL DEFAULT '',
  profile_photo_url text NOT NULL DEFAULT '',
  location geography(Point, 4326),
  onboarding_completed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE TRIGGER users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE refresh_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  family_id uuid NOT NULL,
  token_hash bytea NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  revoked_at timestamptz,
  replaced_by uuid REFERENCES refresh_tokens(id),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_idx ON refresh_tokens(user_id, created_at DESC);

CREATE TABLE pets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  pet_type text NOT NULL,
  breed text NOT NULL DEFAULT '',
  birth_date date,
  age_label text NOT NULL DEFAULT '',
  gender text NOT NULL DEFAULT 'unknown' CHECK (gender IN ('male', 'female', 'unknown')),
  weight_kg numeric(7,2),
  bio text NOT NULL DEFAULT '',
  personality text[] NOT NULL DEFAULT '{}',
  interests text[] NOT NULL DEFAULT '{}',
  primary_image_url text NOT NULL DEFAULT '',
  location geography(Point, 4326),
  is_verified boolean NOT NULL DEFAULT false,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'adopted', 'deleted')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE TRIGGER pets_set_updated_at
BEFORE UPDATE ON pets
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX pets_owner_idx ON pets(owner_id) WHERE deleted_at IS NULL;
CREATE INDEX pets_location_idx ON pets USING gist(location);
CREATE INDEX pets_type_breed_idx ON pets(pet_type, breed) WHERE status = 'active';

CREATE TABLE follows (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  pet_id uuid NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, pet_id)
);

CREATE TABLE posts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  pet_id uuid NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
  author_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind text NOT NULL DEFAULT 'post' CHECK (kind IN ('post', 'reel')),
  caption text NOT NULL DEFAULT '',
  location_name text NOT NULL DEFAULT '',
  media_url text NOT NULL,
  media_type text NOT NULL DEFAULT 'image' CHECK (media_type IN ('image', 'video')),
  visibility text NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'followers')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE TRIGGER posts_set_updated_at
BEFORE UPDATE ON posts
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX posts_feed_idx ON posts(created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX posts_pet_idx ON posts(pet_id, created_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE post_likes (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, post_id)
);

CREATE TABLE post_saves (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, post_id)
);

CREATE TABLE comments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  post_id uuid NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  body text NOT NULL CHECK (length(body) BETWEEN 1 AND 2000),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE INDEX comments_post_idx ON comments(post_id, created_at ASC) WHERE deleted_at IS NULL;

CREATE TABLE stories (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  pet_id uuid NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
  author_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  media_url text NOT NULL,
  media_type text NOT NULL DEFAULT 'image' CHECK (media_type IN ('image', 'video')),
  text_overlay jsonb NOT NULL DEFAULT '{}',
  expires_at timestamptz NOT NULL DEFAULT (now() + interval '24 hours'),
  created_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz
);

CREATE INDEX stories_active_idx ON stories(expires_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE pet_swipes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  source_pet_id uuid NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
  target_pet_id uuid NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
  decision text NOT NULL CHECK (decision IN ('skip', 'like', 'super_like')),
  client_request_id uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (source_pet_id <> target_pet_id),
  UNIQUE (source_pet_id, client_request_id),
  UNIQUE (source_pet_id, target_pet_id)
);

CREATE TABLE matches (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  pet_low_id uuid NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
  pet_high_id uuid NOT NULL REFERENCES pets(id) ON DELETE CASCADE,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'unmatched', 'blocked')),
  matched_at timestamptz NOT NULL DEFAULT now(),
  unmatched_at timestamptz,
  CHECK (pet_low_id < pet_high_id),
  UNIQUE (pet_low_id, pet_high_id)
);

CREATE TABLE chats (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  match_id uuid NOT NULL UNIQUE REFERENCES matches(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE chat_members (
  chat_id uuid NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  last_read_at timestamptz,
  joined_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (chat_id, user_id)
);

CREATE TABLE messages (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  chat_id uuid NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
  sender_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  sender_pet_id uuid REFERENCES pets(id) ON DELETE SET NULL,
  client_message_id uuid NOT NULL,
  message_type text NOT NULL DEFAULT 'text' CHECK (message_type IN ('text', 'image', 'video', 'audio', 'location')),
  body text NOT NULL DEFAULT '',
  media_url text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  edited_at timestamptz,
  deleted_at timestamptz,
  UNIQUE (chat_id, sender_user_id, client_message_id)
);

CREATE INDEX messages_chat_idx ON messages(chat_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;

CREATE TABLE communities (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL UNIQUE,
  description text NOT NULL DEFAULT '',
  emoji text NOT NULL DEFAULT '🐾',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE community_members (
  community_id uuid NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role text NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'moderator', 'member')),
  joined_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (community_id, user_id)
);

CREATE TABLE events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  creator_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  community_id uuid REFERENCES communities(id) ON DELETE SET NULL,
  title text NOT NULL,
  description text NOT NULL DEFAULT '',
  location_name text NOT NULL,
  location geography(Point, 4326),
  starts_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE event_rsvps (
  event_id uuid NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status text NOT NULL DEFAULT 'going' CHECK (status IN ('going', 'interested')),
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (event_id, user_id)
);

CREATE TABLE adoption_listings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL,
  pet_type text NOT NULL,
  breed text NOT NULL DEFAULT '',
  age_label text NOT NULL DEFAULT '',
  gender text NOT NULL DEFAULT 'unknown',
  description text NOT NULL DEFAULT '',
  city text NOT NULL DEFAULT '',
  image_url text NOT NULL DEFAULT '',
  status text NOT NULL DEFAULT 'available' CHECK (status IN ('available', 'pending', 'adopted', 'closed')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE adoption_saves (
  listing_id uuid NOT NULL REFERENCES adoption_listings(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (listing_id, user_id)
);

CREATE TABLE notifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  notification_type text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}',
  read_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notifications_user_idx ON notifications(user_id, created_at DESC);

-- Demo/sample data used to live here as INSERT statements. Per ADR 0006
-- (docs/adr/0006-migration-strategy.md), seed data is never part of a
-- numbered migration: it now lives in server/cmd/seed, a standalone command
-- that refuses to run outside APP_ENV=local (or =test). Run it explicitly
-- with `go run ./cmd/seed`, or let scripts/run-api.ps1 run it for you.
