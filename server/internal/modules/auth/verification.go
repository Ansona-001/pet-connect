package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"petconnect/server/internal/platform/authjwt"
	"petconnect/server/internal/platform/httpx"
)

const (
	verificationTokenTTL = 24 * time.Hour
	resendCooldownWindow = 60 * time.Second
)

type verifyEmailRequest struct {
	Token string `json:"token"`
}

type resendVerificationRequest struct {
	Email string `json:"email"`
}

// oneTimeTokenStatus is the outcome of checking a stored one-time token
// (email verification here; password reset from Milestone 1 Day 11 onward
// follows the same shape) against the current time. Kept as a pure
// function of its inputs — found/usedAt/expiresAt/now — so the happy
// path, expiry, and reuse cases are unit-testable without a database; see
// verification_test.go.
type oneTimeTokenStatus int

const (
	tokenValid oneTimeTokenStatus = iota
	tokenNotFound
	tokenAlreadyUsed
	tokenExpired
)

func evaluateOneTimeToken(now time.Time, found bool, usedAt *time.Time, expiresAt time.Time) oneTimeTokenStatus {
	if !found {
		return tokenNotFound
	}
	if usedAt != nil {
		return tokenAlreadyUsed
	}
	if !expiresAt.After(now) {
		return tokenExpired
	}
	return tokenValid
}

func (h *Handler) verifyEmail(c *fiber.Ctx) error {
	var input verifyEmailRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	input.Token = strings.TrimSpace(input.Token)
	if input.Token == "" {
		return validationProblem(c, "token", "A verification token is required.")
	}

	now := h.clock().UTC()
	hash := authjwt.HashOpaqueToken(input.Token)

	tx, err := h.db.BeginTx(c.UserContext(), pgx.TxOptions{})
	if err != nil {
		return internalProblem(c)
	}
	defer func() { _ = tx.Rollback(c.UserContext()) }()

	var id uuid.UUID
	var usedAt *time.Time
	var expiresAt time.Time
	err = tx.QueryRow(c.UserContext(), `
		SELECT id, used_at, expires_at FROM email_verifications
		WHERE token_hash = $1
		FOR UPDATE`, hash).Scan(&id, &usedAt, &expiresAt)
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return internalProblem(c)
	}

	switch evaluateOneTimeToken(now, found, usedAt, expiresAt) {
	case tokenNotFound, tokenAlreadyUsed, tokenExpired:
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_verification_token",
			"This verification link is invalid or has expired. Request a new one.")
	}

	if _, err := tx.Exec(c.UserContext(), `
		UPDATE email_verifications SET used_at = $2 WHERE id = $1`, id, now); err != nil {
		return internalProblem(c)
	}
	if err := tx.Commit(c.UserContext()); err != nil {
		return internalProblem(c)
	}

	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) resendVerification(c *fiber.Ctx) error {
	var input resendVerificationRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	email := normalizeEmail(input.Email)
	if email == "" {
		return validationProblem(c, "email", "Enter a valid email address.")
	}

	// Every branch below returns this same response — resend-by-email must
	// not reveal whether the account exists, is already verified, or is
	// cooling down (brief §13: "Sensitive auth errors do not reveal
	// whether an account exists").
	respondGeneric := func() error {
		return httpx.OK(c, fiber.Map{
			"message": "If an account exists for that email, a verification link has been sent.",
		})
	}

	var userID uuid.UUID
	err := h.db.QueryRow(c.UserContext(), `
		SELECT id FROM users WHERE email = $1 AND deleted_at IS NULL`, email).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return respondGeneric()
		}
		return internalProblem(c)
	}

	now := h.clock().UTC()

	// A per-account cooldown standing in for the distributed rate limiting
	// Milestone 1 schedules as its own day (brief defect #5) — this
	// refuses a new token while a recent unused one is still live, rather
	// than applying no throttling at all in the meantime.
	var recentCount int
	if err := h.db.QueryRow(c.UserContext(), `
		SELECT count(*) FROM email_verifications
		WHERE user_id = $1 AND used_at IS NULL AND created_at > $2`,
		userID, now.Add(-resendCooldownWindow)).Scan(&recentCount); err != nil {
		return internalProblem(c)
	}
	if recentCount > 0 {
		return respondGeneric()
	}

	plain, err := h.issueEmailVerification(c.UserContext(), h.db, userID, now)
	if err != nil {
		return internalProblem(c)
	}
	_ = h.mailer.SendVerificationEmail(email, plain)

	return respondGeneric()
}

// dbExecer is satisfied by both *pgxpool.Pool and pgx.Tx, so
// issueEmailVerification can run standalone (resendVerification) or as
// part of a larger transaction (register, so the verification row is
// created atomically with the account).
type dbExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (h *Handler) issueEmailVerification(ctx context.Context, db dbExecer, userID uuid.UUID, now time.Time) (string, error) {
	plain, hash, err := authjwt.NewOpaqueToken()
	if err != nil {
		return "", err
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO email_verifications (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`, userID, hash, now.Add(verificationTokenTTL)); err != nil {
		return "", err
	}
	return plain, nil
}
