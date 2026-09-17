package auth

import (
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"petconnect/server/internal/platform/httpx"
)

type sessionResponse struct {
	ID         uuid.UUID `json:"id"`
	DeviceName string    `json:"device_name"`
	Platform   string    `json:"platform"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	IsCurrent  bool      `json:"is_current"`
}

// RegisterProtectedRoutes mounts routes that require authentication. The
// supplied router must already use authentication middleware — see
// cmd/api/main.go, which mounts this alongside RegisterRoutes' public
// /auth/* routes but on the protected router group.
func (h *Handler) RegisterProtectedRoutes(router fiber.Router) {
	router.Get("/sessions", h.listSessions)
	router.Delete("/sessions/:sessionId", h.revokeSession)
	router.Post("/auth/logout-all", h.logoutAll)
}

func (h *Handler) listSessions(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "Authentication is required.")
	}

	// Used only to flag which row is_current in the response — a failure
	// to resolve it just means no row gets flagged, not an error.
	var currentFamilyID uuid.UUID
	if refreshRecordID, err := httpx.RefreshRecordID(c); err == nil {
		_ = h.db.QueryRow(c.UserContext(), `
			SELECT family_id FROM refresh_tokens WHERE id = $1`, refreshRecordID).Scan(&currentFamilyID)
	}

	rows, err := h.db.Query(c.UserContext(), `
		SELECT s.id, s.family_id, COALESCE(d.name, 'Unknown device'), COALESCE(d.platform, ''),
		       s.created_at, s.last_used_at
		FROM sessions s
		LEFT JOIN devices d ON d.id = s.device_id
		WHERE s.user_id = $1 AND s.revoked_at IS NULL
		ORDER BY s.last_used_at DESC`, userID)
	if err != nil {
		return internalProblem(c)
	}
	defer rows.Close()

	sessions := make([]sessionResponse, 0)
	for rows.Next() {
		var s sessionResponse
		var familyID uuid.UUID
		if err := rows.Scan(&s.ID, &familyID, &s.DeviceName, &s.Platform, &s.CreatedAt, &s.LastUsedAt); err != nil {
			return internalProblem(c)
		}
		s.IsCurrent = currentFamilyID != uuid.Nil && familyID == currentFamilyID
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return internalProblem(c)
	}
	return httpx.OK(c, sessions)
}

func (h *Handler) revokeSession(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "Authentication is required.")
	}
	sessionID, err := httpx.UUIDParam(c, "sessionId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_session_id", "The session id is invalid.")
	}

	tx, err := h.db.BeginTx(c.UserContext(), pgx.TxOptions{})
	if err != nil {
		return internalProblem(c)
	}
	defer func() { _ = tx.Rollback(c.UserContext()) }()

	// Scoped to user_id = $2, not just id = $1: this is the BOLA check —
	// without it, any authenticated user could revoke any other user's
	// session by guessing/enumerating session ids.
	var familyID uuid.UUID
	err = tx.QueryRow(c.UserContext(), `
		SELECT family_id FROM sessions
		WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
		FOR UPDATE`, sessionID, userID).Scan(&familyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.Problem(c, fiber.StatusNotFound, "session_not_found", "That session was not found.")
	}
	if err != nil {
		return internalProblem(c)
	}

	now := h.clock().UTC()
	if _, err := tx.Exec(c.UserContext(), `
		UPDATE sessions SET revoked_at = $2 WHERE id = $1`, sessionID, now); err != nil {
		return internalProblem(c)
	}
	if _, err := tx.Exec(c.UserContext(), `
		UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, $2)
		WHERE family_id = $1 AND revoked_at IS NULL`, familyID, now); err != nil {
		return internalProblem(c)
	}
	if err := tx.Commit(c.UserContext()); err != nil {
		return internalProblem(c)
	}

	// Deliberately no Redis invalidation-marker write here — see
	// httpx.SessionInvalidationKey's doc comment: an already-issued access
	// token for this one session is accepted to expire naturally within
	// its short TTL (the same documented latency plain logout has).
	// logout-all is where the stronger, near-immediate guarantee applies.
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) logoutAll(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "Authentication is required.")
	}

	now := h.clock().UTC()
	tx, err := h.db.BeginTx(c.UserContext(), pgx.TxOptions{})
	if err != nil {
		return internalProblem(c)
	}
	defer func() { _ = tx.Rollback(c.UserContext()) }()

	if _, err := tx.Exec(c.UserContext(), `
		UPDATE sessions SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL`, userID, now); err != nil {
		return internalProblem(c)
	}
	if _, err := tx.Exec(c.UserContext(), `
		UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, $2)
		WHERE user_id = $1 AND revoked_at IS NULL`, userID, now); err != nil {
		return internalProblem(c)
	}
	if err := tx.Commit(c.UserContext()); err != nil {
		return internalProblem(c)
	}

	// Near-immediate access-token invalidation across every session (ADR
	// 0004) — without this, an access token issued moments before
	// logout-all would keep working until its natural TTL expiry despite
	// "signed out everywhere." A failure here doesn't fail the request:
	// sessions and refresh tokens are already durably revoked, so the
	// worst case is slower (TTL-bound, not immediate) access-token
	// invalidation, not a security hole.
	invalidatedAfter := now.Format(time.RFC3339Nano)
	if err := h.redis.Set(c.UserContext(), httpx.SessionInvalidationKey(userID), invalidatedAfter, 30*24*time.Hour).Err(); err != nil {
		slog.Error("set session invalidation marker", "error", err, "user_id", userID)
	}

	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}
