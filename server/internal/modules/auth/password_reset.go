package auth

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"petconnect/server/internal/platform/authjwt"
	"petconnect/server/internal/platform/httpx"
)

// Shorter than verificationTokenTTL: an unclaimed password reset link is a
// standing risk (whoever holds it can take over the account) in a way an
// unclaimed email-verification link isn't, so it expires sooner.
const passwordResetTokenTTL = time.Hour

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (h *Handler) forgotPassword(c *fiber.Ctx) error {
	var input forgotPasswordRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	email := normalizeEmail(input.Email)
	if email == "" {
		return validationProblem(c, "email", "Enter a valid email address.")
	}

	// Every branch below returns this same response — like
	// resend-verification, forgot-password must not reveal whether the
	// account exists (brief §13).
	respondGeneric := func() error {
		return httpx.OK(c, fiber.Map{
			"message": "If an account exists for that email, a password reset link has been sent.",
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

	// Same per-account cooldown standing in for real rate limiting as
	// resend-verification (brief defect #5, Milestone 1's dedicated day).
	var recentCount int
	if err := h.db.QueryRow(c.UserContext(), `
		SELECT count(*) FROM password_resets
		WHERE user_id = $1 AND used_at IS NULL AND created_at > $2`,
		userID, now.Add(-resendCooldownWindow)).Scan(&recentCount); err != nil {
		return internalProblem(c)
	}
	if recentCount > 0 {
		return respondGeneric()
	}

	plain, hash, err := authjwt.NewOpaqueToken()
	if err != nil {
		return internalProblem(c)
	}
	if _, err := h.db.Exec(c.UserContext(), `
		INSERT INTO password_resets (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`, userID, hash, now.Add(passwordResetTokenTTL)); err != nil {
		return internalProblem(c)
	}
	_ = h.mailer.SendPasswordReset(email, plain)

	return respondGeneric()
}

func (h *Handler) resetPassword(c *fiber.Ctx) error {
	var input resetPasswordRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	input.Token = strings.TrimSpace(input.Token)
	if input.Token == "" {
		return validationProblem(c, "token", "A reset token is required.")
	}
	passwordLength := len([]byte(input.Password))
	if passwordLength < minimumPasswordBytes || passwordLength > maximumPasswordBytes {
		return validationProblem(c, "password", "Password must contain between 8 and 72 bytes.")
	}

	now := h.clock().UTC()
	hash := authjwt.HashOpaqueToken(input.Token)

	tx, err := h.db.BeginTx(c.UserContext(), pgx.TxOptions{})
	if err != nil {
		return internalProblem(c)
	}
	defer func() { _ = tx.Rollback(c.UserContext()) }()

	var id, userID uuid.UUID
	var usedAt *time.Time
	var expiresAt time.Time
	err = tx.QueryRow(c.UserContext(), `
		SELECT pr.id, pr.user_id, pr.used_at, pr.expires_at
		FROM password_resets pr
		JOIN users u ON u.id = pr.user_id
		WHERE pr.token_hash = $1 AND u.deleted_at IS NULL
		FOR UPDATE OF pr`, hash).Scan(&id, &userID, &usedAt, &expiresAt)
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return internalProblem(c)
	}

	switch evaluateOneTimeToken(now, found, usedAt, expiresAt) {
	case tokenNotFound, tokenAlreadyUsed, tokenExpired:
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_reset_token",
			"This password reset link is invalid or has expired. Request a new one.")
	}

	if _, err := tx.Exec(c.UserContext(), `
		UPDATE users SET password_hash = crypt($2, gen_salt('bf', 12)) WHERE id = $1`,
		userID, input.Password); err != nil {
		return internalProblem(c)
	}
	if _, err := tx.Exec(c.UserContext(), `
		UPDATE password_resets SET used_at = $2 WHERE id = $1`, id, now); err != nil {
		return internalProblem(c)
	}
	// A password reset is a security event (brief §13: "password change...
	// revoke the intended sessions") — revoke every refresh token so a
	// stolen session dies along with the compromised password. This is the
	// same revocation logout() already does, just for every session at
	// once rather than one. Already-issued short-lived access JWTs still
	// work until they expire naturally — the same documented limitation
	// logout has (ADR 0004), not a new gap introduced here.
	if _, err := tx.Exec(c.UserContext(), `
		UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, now())
		WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
		return internalProblem(c)
	}
	if err := tx.Commit(c.UserContext()); err != nil {
		return internalProblem(c)
	}

	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}
