-- Adds the auth/session schema the brief calls out as missing (§4):
-- external auth identities, email verification, password reset, and
-- device/session metadata. Additive-only per ADR 0006
-- (docs/adr/0006-migration-strategy.md) — this is the first migration
-- created under that rule, so it must never modify 000001_init.sql.
--
-- Endpoints and handlers that populate/read these tables are separate,
-- later slices (email verification, password reset, session listing,
-- OAuth linking); this migration only establishes the schema.

-- One-time, hashed, expiring email verification tokens (brief §13:
-- "Verification/reset tokens are one-time, hashed at rest, expiring, and
-- rate-limited"). token_hash stores a hash of the token, never the raw
-- value, matching how refresh_tokens already handles this.
CREATE TABLE email_verifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX email_verifications_user_idx ON email_verifications(user_id, created_at DESC);

-- Password reset tokens: same one-time/hashed/expiring shape as above.
CREATE TABLE password_resets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX password_resets_user_idx ON password_resets(user_id, created_at DESC);

-- External OAuth identities (Google, Apple — ADR 0005's recorded default),
-- linked to the PetConnect user they authenticate as. The two UNIQUE
-- constraints both matter: (provider, provider_user_id) stops the same
-- external account from being linked to two different PetConnect users;
-- (user_id, provider) stops one user from linking two accounts on the same
-- provider, keeping "safe account linking" (brief §13) enforceable in SQL
-- rather than only in application code.
CREATE TABLE oauth_identities (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider text NOT NULL CHECK (provider IN ('google', 'apple')),
  provider_user_id text NOT NULL,
  provider_email citext NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (provider, provider_user_id),
  UNIQUE (user_id, provider)
);

-- A device a user has signed in from — the "device" half of the session
-- list Milestone 1 adds (revoke session(s), logout-all). Kept distinct
-- from any future push-notification device-token table (brief §14's
-- separate /push/devices resource, Milestone 5 scope): this table is about
-- recognizing and revoking *sign-ins*, not delivering notifications.
CREATE TABLE devices (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name text NOT NULL DEFAULT '',
  platform text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  last_seen_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX devices_user_idx ON devices(user_id, last_seen_at DESC);

-- A logical login session, shown to the user and individually revocable.
-- Deliberately distinct from refresh_tokens: a refresh token *rotates* on
-- every use (a new row per rotation, all sharing one family_id), but from
-- the user's point of view "signed in on my phone" is a single, durable
-- thing spanning many rotations. sessions.family_id correlates to
-- refresh_tokens.family_id by value, not by foreign key — family_id is
-- deliberately non-unique in refresh_tokens (every rotated token in a
-- family shares it), so it cannot be an FK target. Revoking a session is
-- expected to also revoke every refresh_tokens row sharing its family_id.
CREATE TABLE sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  device_id uuid REFERENCES devices(id) ON DELETE SET NULL,
  family_id uuid NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_used_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz
);

CREATE INDEX sessions_user_idx ON sessions(user_id, last_used_at DESC) WHERE revoked_at IS NULL;
