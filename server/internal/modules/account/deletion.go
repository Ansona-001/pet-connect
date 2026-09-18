package account

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"petconnect/server/internal/platform/httpx"
)

type deleteAccountRequest struct {
	Password string `json:"password"`
}

// deleteAccount soft-deletes and anonymizes the authenticated account
// (brief §3: "Use soft deletion where content/audit/abuse requirements
// need it"). Requires the current password in the body — a destructive,
// irreversible-in-effect action shouldn't be doable with a bearer token
// alone (e.g. one leaked via a compromised device), the same reasoning
// that justifies password confirmation for other high-stakes actions.
//
// What "anonymize" means here: email, name, bio, city, profile photo, and
// location are cleared or replaced (the email specifically becomes a
// unique unreachable placeholder, freeing the original address for reuse
// — a departed user's address isn't reserved forever); password_hash is
// overwritten with random bytes rather than left as a crackable hash for
// an account nobody can log into anyway. The row itself, and content
// rows that reference user_id (posts, comments, matches, etc.), are kept
// for audit/abuse-history purposes per brief §3 — this endpoint does not
// hard-delete or cascade-delete content.
//
// Deliberately out of scope here: auditing every feed/discovery/
// notification query to confirm a deleted owner's content is actually
// invisible everywhere. That cross-cutting enforcement is Milestone 2's
// explicit gate ("Private/blocked/deleted pets cannot be fetched through
// direct IDs, feed, discovery, or search"), not this endpoint's job —
// this endpoint's job is making the source data honestly reflect
// deletion, which the pets.status update below does for pets specifically.
func (h *Handler) deleteAccount(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "Authentication is required.")
	}

	var input deleteAccountRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	if input.Password == "" {
		return validationProblem(c, "password", "Enter your current password to confirm account deletion.")
	}

	tx, err := h.db.BeginTx(c.UserContext(), pgx.TxOptions{})
	if err != nil {
		return internalProblem(c)
	}
	defer func() { _ = tx.Rollback(c.UserContext()) }()

	var confirmedID uuid.UUID
	err = tx.QueryRow(c.UserContext(), `
		SELECT id FROM users
		WHERE id = $1 AND password_hash = crypt($2, password_hash) AND deleted_at IS NULL
		FOR UPDATE`, userID, input.Password).Scan(&confirmedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.Problem(c, fiber.StatusUnauthorized, "invalid_credentials", "The password is incorrect.")
	}
	if err != nil {
		return internalProblem(c)
	}

	placeholderEmail := fmt.Sprintf("deleted-%s@deleted.petconnect.local", uuid.NewString())
	if _, err := tx.Exec(c.UserContext(), `
		UPDATE users
		SET email = $2,
		    name = '',
		    bio = '',
		    city = '',
		    profile_photo_url = '',
		    location = NULL,
		    password_hash = encode(gen_random_bytes(32), 'hex'),
		    deleted_at = now()
		WHERE id = $1`, userID, placeholderEmail); err != nil {
		return internalProblem(c)
	}

	if _, err := tx.Exec(c.UserContext(), `
		UPDATE pets SET status = 'deleted' WHERE owner_id = $1 AND status <> 'deleted'`, userID); err != nil {
		return internalProblem(c)
	}
	if _, err := tx.Exec(c.UserContext(), `
		UPDATE sessions SET revoked_at = COALESCE(revoked_at, now()) WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
		return internalProblem(c)
	}
	if _, err := tx.Exec(c.UserContext(), `
		UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, now()) WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
		return internalProblem(c)
	}

	if err := tx.Commit(c.UserContext()); err != nil {
		return internalProblem(c)
	}

	// Best-effort, same as logout-all: sessions/tokens are already durably
	// revoked, so a failure here only means already-issued access tokens
	// take until their natural TTL to stop working instead of immediately.
	invalidatedAfter := time.Now().UTC().Format(time.RFC3339Nano)
	if err := h.redis.Set(c.UserContext(), httpx.SessionInvalidationKey(userID), invalidatedAfter, 30*24*time.Hour).Err(); err != nil {
		slog.Error("set session invalidation marker", "error", err, "user_id", userID)
	}

	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}
