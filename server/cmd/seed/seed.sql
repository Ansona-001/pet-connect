-- Local/test-only demo data. Moved out of migrations/000001_init.sql per
-- ADR 0006 (docs/adr/0006-migration-strategy.md) — see cmd/seed/main.go for
-- the environment guard that keeps this out of staging/production.

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
