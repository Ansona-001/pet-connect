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

INSERT INTO users (id, email, password_hash, name, bio, city, profile_photo_url, location, onboarding_completed_at)
VALUES
  ('00000000-0000-4000-8000-000000000001', 'demo@petconnect.local', crypt('PetConnect123!', gen_salt('bf', 10)), 'Alex', 'Pet parent and weekend explorer.', 'Dubai', '/media/demo/onboarding/onboarding_3.jpg', ST_SetSRID(ST_MakePoint(55.2708, 25.2048), 4326)::geography, now()),
  ('00000000-0000-4000-8000-000000000002', 'maya@petconnect.local', crypt('PetConnect123!', gen_salt('bf', 10)), 'Maya', 'Coco''s person.', 'Dubai', '', ST_SetSRID(ST_MakePoint(55.2742, 25.2071), 4326)::geography, now()),
  ('00000000-0000-4000-8000-000000000003', 'noah@petconnect.local', crypt('PetConnect123!', gen_salt('bf', 10)), 'Noah', 'Trail walks with Milo.', 'Dubai', '', ST_SetSRID(ST_MakePoint(55.2850, 25.2150), 4326)::geography, now()),
  ('00000000-0000-4000-8000-000000000004', 'sara@petconnect.local', crypt('PetConnect123!', gen_salt('bf', 10)), 'Sara', 'Simba runs the house.', 'Dubai', '', ST_SetSRID(ST_MakePoint(55.3000, 25.2100), 4326)::geography, now())
ON CONFLICT (email) DO NOTHING;

INSERT INTO pets (id, owner_id, name, pet_type, breed, age_label, gender, bio, personality, interests, primary_image_url, location, is_verified)
VALUES
  ('10000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000001', 'Luna', 'dog', 'Golden Retriever', '3 yrs', 'female', 'Sunshine chaser and professional friend-maker.', ARRAY['Friendly','Playful','Energetic'], ARRAY['Playdates','Parks','Training'], '/media/demo/onboarding/onboarding_1.jpg', ST_SetSRID(ST_MakePoint(55.2708, 25.2048), 4326)::geography, true),
  ('10000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000002', 'Coco', 'dog', 'Pembroke Corgi', '2 yrs', 'female', 'Tiny legs, huge park energy.', ARRAY['Playful','Social','Gentle'], ARRAY['Playdates','Parks'], '/media/demo/content/pet_playdate.png', ST_SetSRID(ST_MakePoint(55.2742, 25.2071), 4326)::geography, true),
  ('10000000-0000-4000-8000-000000000003', '00000000-0000-4000-8000-000000000003', 'Milo', 'dog', 'Shetland Sheepdog', '3 yrs', 'male', 'Trail runner and ball collector.', ARRAY['Energetic','Friendly','Curious'], ARRAY['Parks','Training'], '/media/demo/onboarding/onboarding_2.jpg', ST_SetSRID(ST_MakePoint(55.2850, 25.2150), 4326)::geography, true),
  ('10000000-0000-4000-8000-000000000004', '00000000-0000-4000-8000-000000000004', 'Simba', 'cat', 'British Shorthair', '4 yrs', 'male', 'Window watcher and professional slow-blinker.', ARRAY['Calm','Cuddly','Independent'], ARRAY['Pet-friendly cafés','Health & wellness'], '/media/demo/onboarding/onboarding_3.jpg', ST_SetSRID(ST_MakePoint(55.3000, 25.2100), 4326)::geography, true)
ON CONFLICT (id) DO NOTHING;

INSERT INTO posts (id, pet_id, author_user_id, kind, caption, location_name, media_url, media_type, created_at)
VALUES
  ('20000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000002', 'post', 'When a quick hello turns into the best playdate ever. Same time tomorrow? 🐾', 'Al Barsha Pond Park', '/media/demo/content/pet_playdate.png', 'image', now() - interval '18 minutes'),
  ('20000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000003', '00000000-0000-4000-8000-000000000003', 'reel', 'Fresh grass, open trails, and absolutely no intention of going home.', 'Mushrif Park', '/media/demo/onboarding/onboarding_2.jpg', 'image', now() - interval '1 hour'),
  ('20000000-0000-4000-8000-000000000003', '10000000-0000-4000-8000-000000000004', '00000000-0000-4000-8000-000000000004', 'post', 'Today''s agenda: one cuddle, three naps, zero meetings.', 'Downtown Dubai', '/media/demo/onboarding/onboarding_3.jpg', 'image', now() - interval '3 hours')
ON CONFLICT (id) DO NOTHING;

INSERT INTO post_likes (user_id, post_id)
SELECT u.id, p.id
FROM users u CROSS JOIN posts p
WHERE u.id <> p.author_user_id
ON CONFLICT DO NOTHING;

INSERT INTO stories (id, pet_id, author_user_id, media_url, media_type, expires_at)
VALUES
  ('30000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000002', '/media/demo/content/pet_playdate.png', 'image', now() + interval '24 hours'),
  ('30000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000003', '00000000-0000-4000-8000-000000000003', '/media/demo/onboarding/onboarding_2.jpg', 'image', now() + interval '24 hours'),
  ('30000000-0000-4000-8000-000000000003', '10000000-0000-4000-8000-000000000004', '00000000-0000-4000-8000-000000000004', '/media/demo/onboarding/onboarding_3.jpg', 'image', now() + interval '24 hours')
ON CONFLICT (id) DO NOTHING;

INSERT INTO pet_swipes (id, source_pet_id, target_pet_id, decision, client_request_id)
VALUES ('40000000-0000-4000-8000-000000000001', '10000000-0000-4000-8000-000000000002', '10000000-0000-4000-8000-000000000001', 'like', '40000000-0000-4000-8000-000000000011')
ON CONFLICT DO NOTHING;

INSERT INTO communities (id, owner_user_id, name, description, emoji)
VALUES
  ('50000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000002', 'Dubai Dog Parents', 'Walks, tips, and meetups for Dubai dog families.', '🐕'),
  ('50000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000004', 'Cat People UAE', 'A calm corner for the cat community.', '🐈')
ON CONFLICT (name) DO NOTHING;

INSERT INTO events (id, creator_user_id, community_id, title, description, location_name, location, starts_at)
VALUES
  ('60000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000002', '50000000-0000-4000-8000-000000000001', 'Sunset social walk', 'A relaxed social loop for friendly dogs.', 'Dubai Hills Dog Park', ST_SetSRID(ST_MakePoint(55.2500, 25.1100), 4326)::geography, now() + interval '3 days'),
  ('60000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000003', NULL, 'Adoption open day', 'Meet pets looking for a permanent home.', 'The Petshop, DIP', ST_SetSRID(ST_MakePoint(55.1600, 24.9900), 4326)::geography, now() + interval '4 days')
ON CONFLICT (id) DO NOTHING;

INSERT INTO adoption_listings (id, owner_user_id, name, pet_type, breed, age_label, gender, description, city, image_url)
VALUES
  ('70000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000003', 'Buddy', 'dog', 'Mixed breed', '1 yr', 'male', 'Gentle, vaccinated, and ready for an active family.', 'Dubai', '/media/demo/onboarding/onboarding_2.jpg'),
  ('70000000-0000-4000-8000-000000000002', '00000000-0000-4000-8000-000000000004', 'Mimi', 'cat', 'Domestic Shorthair', '2 yrs', 'female', 'A quiet companion who loves sunny windows.', 'Dubai', '/media/demo/onboarding/onboarding_3.jpg')
ON CONFLICT (id) DO NOTHING;
