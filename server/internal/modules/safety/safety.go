// Package safety implements the block/mute/report primitives the brief
// flags as entirely missing (docs/requirements-traceability.md). These
// are user-scoped, not pet-scoped — see migrations/000004's doc comment
// for why, echoing ADR 0001's reasoning that user_id is the durable
// moderation anchor.
package safety

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"petconnect/server/internal/platform/httpx"
)

type Handler struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

// RegisterRoutes mounts safety routes on an authenticated router rooted at /v1.
func RegisterRoutes(router fiber.Router, db *pgxpool.Pool) {
	New(db).RegisterRoutes(router)
}

// RegisterRoutes mounts routes. The supplied router must already use authentication middleware.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/blocks", h.listBlocks)
	router.Post("/blocks", h.createBlock)
	router.Delete("/blocks/:userId", h.deleteBlock)

	router.Get("/mutes", h.listMutes)
	router.Post("/mutes", h.createMute)
	router.Delete("/mutes/:userId", h.deleteMute)

	router.Post("/reports", h.createReport)
}

type relatedUserRequest struct {
	UserID uuid.UUID `json:"user_id"`
}

type relatedUser struct {
	UserID    uuid.UUID `json:"user_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Handler) listBlocks(c *fiber.Ctx) error {
	return h.listRelated(c, "blocks", "blocker_user_id", "blocked_user_id")
}

func (h *Handler) listMutes(c *fiber.Ctx) error {
	return h.listRelated(c, "mutes", "muter_user_id", "muted_user_id")
}

func (h *Handler) listRelated(c *fiber.Ctx, table, ownerColumn, targetColumn string) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return authenticationProblem(c)
	}
	// table/ownerColumn/targetColumn are fixed, internal call-site
	// constants (never request input), so building the query with them
	// via string concatenation carries no injection risk.
	rows, err := h.db.Query(c.UserContext(), `
		SELECT t.`+targetColumn+`, u.name, t.created_at
		FROM `+table+` t
		JOIN users u ON u.id = t.`+targetColumn+`
		WHERE t.`+ownerColumn+` = $1
		ORDER BY t.created_at DESC`, userID)
	if err != nil {
		return internalProblem(c)
	}
	defer rows.Close()

	items := make([]relatedUser, 0)
	for rows.Next() {
		var item relatedUser
		if err := rows.Scan(&item.UserID, &item.Name, &item.CreatedAt); err != nil {
			return internalProblem(c)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return internalProblem(c)
	}
	return httpx.OK(c, items)
}

func (h *Handler) createBlock(c *fiber.Ctx) error {
	blockerID, err := httpx.UserID(c)
	if err != nil {
		return authenticationProblem(c)
	}
	var input relatedUserRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	if input.UserID == uuid.Nil {
		return validationProblem(c, "user_id", "user_id is required.")
	}
	if input.UserID == blockerID {
		return validationProblem(c, "user_id", "You cannot block yourself.")
	}

	var targetExists bool
	if err := h.db.QueryRow(c.UserContext(), `
		SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)`, input.UserID).Scan(&targetExists); err != nil {
		return internalProblem(c)
	}
	if !targetExists {
		return httpx.Problem(c, fiber.StatusNotFound, "user_not_found", "That user could not be found.")
	}

	tx, err := h.db.BeginTx(c.UserContext(), pgx.TxOptions{})
	if err != nil {
		return internalProblem(c)
	}
	defer func() { _ = tx.Rollback(c.UserContext()) }()

	if _, err := tx.Exec(c.UserContext(), `
		INSERT INTO blocks (blocker_user_id, blocked_user_id)
		VALUES ($1, $2)
		ON CONFLICT (blocker_user_id, blocked_user_id) DO NOTHING`, blockerID, input.UserID); err != nil {
		return internalProblem(c)
	}

	// Sever any existing match(es) between the two users' pets. The chat
	// module already treats a match's 'blocked' status as terminal —
	// ListChats filters it out and ensureMembership rejects sending —
	// so setting this status is the entire integration needed to make a
	// block immediately affect chat; no chat module changes required.
	if _, err := tx.Exec(c.UserContext(), `
		UPDATE matches m
		SET status = 'blocked'
		WHERE m.status = 'active'
		  AND EXISTS (
		    SELECT 1 FROM pets pl, pets ph
		    WHERE pl.id = m.pet_low_id AND ph.id = m.pet_high_id
		      AND ((pl.owner_id = $1 AND ph.owner_id = $2) OR (pl.owner_id = $2 AND ph.owner_id = $1))
		  )`, blockerID, input.UserID); err != nil {
		return internalProblem(c)
	}

	if err := tx.Commit(c.UserContext()); err != nil {
		return internalProblem(c)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}

// deleteBlock removes the block row but deliberately does not restore any
// match this block severed back to 'active' — a match that reappears
// silently after an unblock, without a fresh mutual swipe, would surprise
// the other party with a chat thread they never agreed to reopen.
func (h *Handler) deleteBlock(c *fiber.Ctx) error {
	blockerID, err := httpx.UserID(c)
	if err != nil {
		return authenticationProblem(c)
	}
	targetID, err := httpx.UUIDParam(c, "userId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_user_id", "user id must be a valid UUID.")
	}
	if _, err := h.db.Exec(c.UserContext(), `
		DELETE FROM blocks WHERE blocker_user_id = $1 AND blocked_user_id = $2`, blockerID, targetID); err != nil {
		return internalProblem(c)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) createMute(c *fiber.Ctx) error {
	muterID, err := httpx.UserID(c)
	if err != nil {
		return authenticationProblem(c)
	}
	var input relatedUserRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	if input.UserID == uuid.Nil {
		return validationProblem(c, "user_id", "user_id is required.")
	}
	if input.UserID == muterID {
		return validationProblem(c, "user_id", "You cannot mute yourself.")
	}

	var targetExists bool
	if err := h.db.QueryRow(c.UserContext(), `
		SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)`, input.UserID).Scan(&targetExists); err != nil {
		return internalProblem(c)
	}
	if !targetExists {
		return httpx.Problem(c, fiber.StatusNotFound, "user_not_found", "That user could not be found.")
	}

	if _, err := h.db.Exec(c.UserContext(), `
		INSERT INTO mutes (muter_user_id, muted_user_id)
		VALUES ($1, $2)
		ON CONFLICT (muter_user_id, muted_user_id) DO NOTHING`, muterID, input.UserID); err != nil {
		return internalProblem(c)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) deleteMute(c *fiber.Ctx) error {
	muterID, err := httpx.UserID(c)
	if err != nil {
		return authenticationProblem(c)
	}
	targetID, err := httpx.UUIDParam(c, "userId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_user_id", "user id must be a valid UUID.")
	}
	if _, err := h.db.Exec(c.UserContext(), `
		DELETE FROM mutes WHERE muter_user_id = $1 AND muted_user_id = $2`, muterID, targetID); err != nil {
		return internalProblem(c)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

var allowedSubjectTypes = map[string]struct{}{
	"user": {}, "pet": {}, "post": {}, "comment": {}, "story": {}, "message": {},
}

var allowedReportReasons = map[string]struct{}{
	"spam": {}, "harassment": {}, "inappropriate_content": {},
	"fake_profile": {}, "animal_welfare": {}, "other": {},
}

const maxReportDetailsLength = 2000

type reportRequest struct {
	SubjectType string    `json:"subject_type"`
	SubjectID   uuid.UUID `json:"subject_id"`
	Reason      string    `json:"reason"`
	Details     string    `json:"details"`
}

type fieldError struct {
	field   string
	message string
}

func normalizeReportInput(input reportRequest) reportRequest {
	input.SubjectType = strings.ToLower(strings.TrimSpace(input.SubjectType))
	input.Reason = strings.ToLower(strings.TrimSpace(input.Reason))
	input.Details = strings.TrimSpace(input.Details)
	return input
}

func validateReportInput(input reportRequest) *fieldError {
	if _, ok := allowedSubjectTypes[input.SubjectType]; !ok {
		return &fieldError{"subject_type", "subject_type must be user, pet, post, comment, story, or message."}
	}
	if input.SubjectID == uuid.Nil {
		return &fieldError{"subject_id", "subject_id is required."}
	}
	if _, ok := allowedReportReasons[input.Reason]; !ok {
		return &fieldError{"reason", "reason must be spam, harassment, inappropriate_content, fake_profile, animal_welfare, or other."}
	}
	if len([]rune(input.Details)) > maxReportDetailsLength {
		return &fieldError{"details", "details cannot exceed 2000 characters."}
	}
	return nil
}

// createReport is deliberately not rate-limited here — the "Rate limiting
// & BOLA coverage" day covers Redis-backed limits for auth endpoints
// specifically; extending that to /reports is separate follow-up work,
// not part of these primitives.
func (h *Handler) createReport(c *fiber.Ctx) error {
	reporterID, err := httpx.UserID(c)
	if err != nil {
		return authenticationProblem(c)
	}
	var input reportRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	input = normalizeReportInput(input)
	if fieldErr := validateReportInput(input); fieldErr != nil {
		return validationProblem(c, fieldErr.field, fieldErr.message)
	}

	reportedUserID, err := resolveReportedUser(c.UserContext(), h.db, input.SubjectType, input.SubjectID)
	if errors.Is(err, errSubjectNotFound) {
		return httpx.Problem(c, fiber.StatusNotFound, "subject_not_found", "The reported content could not be found.")
	}
	if err != nil {
		return internalProblem(c)
	}
	if reportedUserID == reporterID {
		return validationProblem(c, "subject_id", "You cannot report your own content.")
	}

	if _, err := h.db.Exec(c.UserContext(), `
		INSERT INTO reports (reporter_user_id, reported_user_id, subject_type, subject_id, reason, details)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		reporterID, reportedUserID, input.SubjectType, input.SubjectID, input.Reason, input.Details); err != nil {
		return internalProblem(c)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}

var errSubjectNotFound = errors.New("report subject not found")

// resolveReportedUser looks up the accountable user behind a report's
// subject, so `reports.reported_user_id` can be populated at write time
// (see migrations/000004's doc comment on why that's stored redundantly).
func resolveReportedUser(ctx context.Context, db *pgxpool.Pool, subjectType string, subjectID uuid.UUID) (uuid.UUID, error) {
	var query string
	switch subjectType {
	case "user":
		query = `SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL`
	case "pet":
		query = `SELECT owner_id FROM pets WHERE id = $1 AND deleted_at IS NULL`
	case "post":
		query = `SELECT author_user_id FROM posts WHERE id = $1 AND deleted_at IS NULL`
	case "comment":
		query = `SELECT user_id FROM comments WHERE id = $1 AND deleted_at IS NULL`
	case "story":
		query = `SELECT author_user_id FROM stories WHERE id = $1 AND deleted_at IS NULL`
	case "message":
		query = `SELECT sender_user_id FROM messages WHERE id = $1 AND deleted_at IS NULL`
	default:
		return uuid.Nil, errSubjectNotFound
	}
	var reportedUserID uuid.UUID
	err := db.QueryRow(ctx, query, subjectID).Scan(&reportedUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, errSubjectNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	return reportedUserID, nil
}

func authenticationProblem(c *fiber.Ctx) error {
	return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
}

func validationProblem(c *fiber.Ctx, field, message string) error {
	return c.Status(fiber.StatusUnprocessableEntity).JSON(httpx.ErrorPayload{Error: httpx.APIError{
		Code: "validation_failed", Message: "One or more fields are invalid.",
		Fields: map[string]string{field: message}, RequestID: c.GetRespHeader(fiber.HeaderXRequestID),
	}})
}

func internalProblem(c *fiber.Ctx) error {
	return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "The request could not be completed.")
}
