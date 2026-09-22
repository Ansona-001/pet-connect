// Package integration holds end-to-end HTTP tests that exercise real
// routes against a real Postgres/Redis, rather than unit-testing SQL or
// handler logic in isolation. This file is the "authorization test
// matrix across every current resource route" (brief Milestone 1's
// final day): for every route an earlier audit identified as owning a
// caller-scoped resource by ID, it drives the actual HTTP endpoint as
// one user trying to act on a different user's resource, and asserts
// the request is rejected — locking in behavior that already exists
// (see each subtest's comment for the exact code location it's
// regression-testing) rather than fixing new bugs, since the audit this
// day is based on found no unscoped route among those that should be
// scoped. The one flagged gap (public, unauthenticated media downloads)
// is a deliberate, already-recorded decision — see ADR 0003 — not an
// oversight, so it isn't tested here as a "failure."
//
// These tests require the same local Docker Compose Postgres/Redis every
// other day's live verification in this project has used and skip
// gracefully when they aren't reachable (in particular, CI's `go` job —
// see .github/workflows/ci.yml — runs with neither service, by design).
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"petconnect/server/internal/modules/account"
	"petconnect/server/internal/modules/auth"
	"petconnect/server/internal/modules/chat"
	"petconnect/server/internal/modules/discovery"
	"petconnect/server/internal/modules/matching"
	"petconnect/server/internal/modules/notifications"
	"petconnect/server/internal/modules/pets"
	"petconnect/server/internal/modules/profile"
	"petconnect/server/internal/modules/safety"
	"petconnect/server/internal/modules/social"
	"petconnect/server/internal/platform/authjwt"
	"petconnect/server/internal/platform/database"
	"petconnect/server/internal/platform/httpx"
	"petconnect/server/internal/platform/mailer"
)

const (
	testDatabaseURL = "postgres://petconnect:petconnect_dev@127.0.0.1:5433/petconnect?sslmode=disable"
	testRedisAddr   = "127.0.0.1:6380"
)

type testEnv struct {
	app    *fiber.App
	db     *pgxpool.Pool
	tokens *authjwt.Manager
}

// setupTestEnv wires a real Fiber app — the same module set and
// middleware order as cmd/api/main.go's protected group, minus media
// (which needs MinIO, irrelevant to authorization testing here) — against
// the local Postgres/Redis. Skips the test (not fails) if either is
// unreachable.
func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	db, err := database.Open(ctx, testDatabaseURL)
	if err != nil {
		t.Skipf("skipping integration test: local Postgres not reachable (%v) — start it with `docker compose up -d`", err)
	}
	t.Cleanup(db.Close)

	redisClient := redis.NewClient(&redis.Options{Addr: testRedisAddr})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping integration test: local Redis not reachable (%v) — start it with `docker compose up -d`", err)
	}
	t.Cleanup(func() { _ = redisClient.Close() })

	tokens := authjwt.New("integration-test-secret-not-for-production", 15*time.Minute)

	app := fiber.New()
	v1 := app.Group("/v1")
	authHandler := auth.RegisterRoutes(v1, db, tokens, 720*time.Hour, mailer.LogSender{}, redisClient, auth.GoogleOAuthConfig{}, auth.AppleOAuthConfig{})
	protected := v1.Group("", httpx.Authenticate(tokens, redisClient))
	authHandler.RegisterProtectedRoutes(protected)
	account.RegisterRoutes(protected, db, redisClient)
	pets.RegisterRoutes(protected, db)
	profile.RegisterRoutes(protected, db)
	social.RegisterRoutes(protected, db)
	discovery.RegisterRoutes(protected, db)
	notifications.RegisterRoutes(protected, db)
	safety.RegisterRoutes(protected, db)
	matching.RegisterRoutes(protected, matching.NewService(db, nil))
	chat.RegisterRoutes(protected, chat.NewService(db, nil))

	return &testEnv{app: app, db: db, tokens: tokens}
}

type testUser struct {
	ID    uuid.UUID
	Token string
	PetID uuid.UUID
}

// createUser inserts a fresh user + one pet directly via SQL (bypassing
// registration for speed/determinism) and issues a real access token for
// them, so subtests exercise httpx.Authenticate exactly as production
// traffic does. Cleanup cascades: deleting the user cascades to their
// pets, sessions, notifications, etc. via the schema's ON DELETE CASCADE.
func (env *testEnv) createUser(t *testing.T, label string) testUser {
	t.Helper()
	ctx := context.Background()
	userID := uuid.New()
	email := fmt.Sprintf("bola-%s-%s@test.local", label, userID.String()[:8])
	if _, err := env.db.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name)
		VALUES ($1, $2, crypt('irrelevant-password', gen_salt('bf', 4)), $3)`,
		userID, email, "BOLA Test "+label); err != nil {
		t.Fatalf("create test user %s: %v", label, err)
	}
	t.Cleanup(func() {
		_, _ = env.db.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	petID := uuid.New()
	if _, err := env.db.Exec(ctx, `
		INSERT INTO pets (id, owner_id, name, pet_type, gender)
		VALUES ($1, $2, $3, 'dog', 'unknown')`, petID, userID, "Pet "+label); err != nil {
		t.Fatalf("create test pet for %s: %v", label, err)
	}

	access, _, err := env.tokens.Issue(userID, uuid.New())
	if err != nil {
		t.Fatalf("issue token for %s: %v", label, err)
	}
	return testUser{ID: userID, Token: access, PetID: petID}
}

// createMatch creates a fresh active match + chat between two users'
// *existing* pets (userA.PetID/userB.PetID), inserting directly via SQL
// to bypass the full swipe flow. Takes explicit pet ids (rather than
// reusing a testUser's default pet) so multiple subtests can each get
// their own match without colliding with `matches`' unique
// (pet_low_id, pet_high_id) constraint.
func (env *testEnv) createMatch(t *testing.T, userX, userY testUser) (chatID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	lowPetID, highPetID := userX.PetID, userY.PetID
	if bytes.Compare(lowPetID[:], highPetID[:]) > 0 {
		lowPetID, highPetID = highPetID, lowPetID
	}
	matchID := uuid.New()
	if _, err := env.db.Exec(ctx, `
		INSERT INTO matches (id, pet_low_id, pet_high_id, status) VALUES ($1, $2, $3, 'active')`,
		matchID, lowPetID, highPetID); err != nil {
		t.Fatalf("create test match: %v", err)
	}
	chatID = uuid.New()
	if _, err := env.db.Exec(ctx, `INSERT INTO chats (id, match_id) VALUES ($1, $2)`, chatID, matchID); err != nil {
		t.Fatalf("create test chat: %v", err)
	}
	if _, err := env.db.Exec(ctx, `
		INSERT INTO chat_members (chat_id, user_id) VALUES ($1, $2), ($1, $3)`,
		chatID, userX.ID, userY.ID); err != nil {
		t.Fatalf("create test chat members: %v", err)
	}
	return chatID
}

// freshPet inserts an additional pet for user, so a test needing "one of
// this user's pets" doesn't collide with another subtest's use of their
// default testUser.PetID (e.g. two different matches for the same user
// pair would otherwise violate matches' unique pet-pair constraint).
func (env *testEnv) freshPet(t *testing.T, owner testUser) uuid.UUID {
	t.Helper()
	petID := uuid.New()
	if _, err := env.db.Exec(context.Background(), `
		INSERT INTO pets (id, owner_id, name, pet_type, gender) VALUES ($1, $2, 'Extra', 'dog', 'unknown')`,
		petID, owner.ID); err != nil {
		t.Fatalf("create fresh pet: %v", err)
	}
	return petID
}

func (env *testEnv) do(t *testing.T, method, path, token string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := env.app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func decodeErrorCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	var payload httpx.ErrorPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	return payload.Error.Code
}

// TestBOLAMatrix is the authorization test matrix: each subtest is one
// user (userA) attempting to read/modify/delete a resource that belongs
// to a different user (userB) by substituting userB's id where userA's
// own would normally go.
func TestBOLAMatrix(t *testing.T) {
	env := setupTestEnv(t)
	userA := env.createUser(t, "a")
	userB := env.createUser(t, "b")

	t.Run("cannot patch another user's pet", func(t *testing.T) {
		// Regression-tests server/internal/modules/pets/pets.go's patch
		// handler: `WHERE id = $1 AND owner_id = $2`.
		resp := env.do(t, http.MethodPatch, "/v1/pets/"+userB.PetID.String(), userA.Token, map[string]any{
			"name": "Hijacked",
		})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
		if code := decodeErrorCode(t, resp); code != "pet_not_found" {
			t.Fatalf("error code = %q, want pet_not_found", code)
		}

		// Sanity check the predicate actually discriminates: the same
		// request against the caller's own pet must succeed.
		resp = env.do(t, http.MethodPatch, "/v1/pets/"+userA.PetID.String(), userA.Token, map[string]any{
			"name": "Renamed",
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("patching own pet: status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("cannot delete another user's pet", func(t *testing.T) {
		// A throwaway third pet so this subtest doesn't consume userB's
		// only pet and interfere with other subtests' ordering.
		ctx := context.Background()
		extraPetID := uuid.New()
		if _, err := env.db.Exec(ctx, `
			INSERT INTO pets (id, owner_id, name, pet_type, gender) VALUES ($1, $2, 'Extra', 'dog', 'unknown')`,
			extraPetID, userB.ID); err != nil {
			t.Fatalf("create extra pet: %v", err)
		}

		resp := env.do(t, http.MethodDelete, "/v1/pets/"+extraPetID.String(), userA.Token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}

		var status string
		if err := env.db.QueryRow(ctx, `SELECT status FROM pets WHERE id = $1`, extraPetID).Scan(&status); err != nil {
			t.Fatalf("check pet survived: %v", err)
		}
		if status == "deleted" {
			t.Fatal("pet was deleted by a non-owner")
		}
	})

	t.Run("cannot revoke another user's session", func(t *testing.T) {
		// Regression-tests auth/sessions.go's revokeSession — the file's
		// own comment calls this out as the concrete BOLA check.
		ctx := context.Background()
		sessionID := uuid.New()
		familyID := uuid.New()
		if _, err := env.db.Exec(ctx, `
			INSERT INTO sessions (id, user_id, family_id) VALUES ($1, $2, $3)`,
			sessionID, userB.ID, familyID); err != nil {
			t.Fatalf("create test session: %v", err)
		}

		resp := env.do(t, http.MethodDelete, "/v1/sessions/"+sessionID.String(), userA.Token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
		if code := decodeErrorCode(t, resp); code != "session_not_found" {
			t.Fatalf("error code = %q, want session_not_found", code)
		}

		var revokedAt *time.Time
		if err := env.db.QueryRow(ctx, `SELECT revoked_at FROM sessions WHERE id = $1`, sessionID).Scan(&revokedAt); err != nil {
			t.Fatalf("check session survived: %v", err)
		}
		if revokedAt != nil {
			t.Fatal("session was revoked by a non-owner")
		}
	})

	t.Run("cannot mark another user's notification read", func(t *testing.T) {
		// Regression-tests notifications.go's readOne: `WHERE id=$1 AND user_id=$2`.
		ctx := context.Background()
		notificationID := uuid.New()
		if _, err := env.db.Exec(ctx, `
			INSERT INTO notifications (id, user_id, notification_type, payload)
			VALUES ($1, $2, 'test', '{}')`, notificationID, userB.ID); err != nil {
			t.Fatalf("create test notification: %v", err)
		}

		resp := env.do(t, http.MethodPut, "/v1/notifications/"+notificationID.String()+"/read", userA.Token, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}

		var readAt *time.Time
		if err := env.db.QueryRow(ctx, `SELECT read_at FROM notifications WHERE id = $1`, notificationID).Scan(&readAt); err != nil {
			t.Fatalf("check notification survived: %v", err)
		}
		if readAt != nil {
			t.Fatal("notification was marked read by a non-owner")
		}
	})

	t.Run("cannot swipe using a pet you don't own", func(t *testing.T) {
		// Regression-tests matching/service.go's ensurePetOwner: the path
		// pet id (source) must belong to the caller.
		resp := env.do(t, http.MethodPost, "/v1/pets/"+userB.PetID.String()+"/swipes", userA.Token, map[string]any{
			"target_pet_id":     userA.PetID.String(),
			"decision":          "like",
			"client_request_id": uuid.NewString(),
		})
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", resp.StatusCode)
		}
		if code := decodeErrorCode(t, resp); code != "pet_forbidden" {
			t.Fatalf("error code = %q, want pet_forbidden", code)
		}
	})

	t.Run("cannot create a post as a pet you don't own", func(t *testing.T) {
		// Regression-tests social/handlers.go's createPostByKind: the
		// insert only succeeds `WHERE p.id=$2 AND p.owner_id=$1`.
		resp := env.do(t, http.MethodPost, "/v1/posts", userA.Token, map[string]any{
			"pet_id":    userB.PetID.String(),
			"caption":   "stolen identity post",
			"media_url": "/media/test.jpg",
		})
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", resp.StatusCode)
		}
		if code := decodeErrorCode(t, resp); code != "pet_not_owned" {
			t.Fatalf("error code = %q, want pet_not_owned", code)
		}
	})

	t.Run("cannot send a chat message as a pet outside the match", func(t *testing.T) {
		// Regression-tests chat/service.go's SendMessage sender_pet_id
		// check: even a legitimate chat member can't speak "as" a pet
		// that isn't part of this specific match.
		chatID := env.createMatch(t, userA, userB)

		// A third user, uninvolved in this match, whose pet userA tries to
		// (mis)represent as the sender.
		userC := env.createUser(t, "c")

		resp := env.do(t, http.MethodPost, "/v1/chats/"+chatID.String()+"/messages", userA.Token, map[string]any{
			"client_message_id": uuid.NewString(),
			"message_type":      "text",
			"body":              "hello, pretending to be someone else's pet",
			"sender_pet_id":     userC.PetID.String(),
		})
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", resp.StatusCode)
		}
		if code := decodeErrorCode(t, resp); code != "sender_pet_forbidden" {
			t.Fatalf("error code = %q, want sender_pet_forbidden", code)
		}
	})

	t.Run("a stranger cannot send messages into a match they aren't part of", func(t *testing.T) {
		// A fresh pet pair, distinct from the previous subtest's match, so
		// this doesn't collide with matches' unique (pet_low_id,
		// pet_high_id) constraint.
		userAOtherPet := env.freshPet(t, userA)
		chatID := env.createMatch(t, testUser{ID: userA.ID, PetID: userAOtherPet}, userB)

		stranger := env.createUser(t, "stranger")
		resp := env.do(t, http.MethodPost, "/v1/chats/"+chatID.String()+"/messages", stranger.Token, map[string]any{
			"client_message_id": uuid.NewString(),
			"message_type":      "text",
			"body":              "I shouldn't be able to send this",
		})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
	})
}
