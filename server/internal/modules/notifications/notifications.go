package notifications

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"petconnect/server/internal/platform/httpx"
)

const (
	defaultLimit = 50
	maximumLimit = 100
)

// Handler exposes an authenticated user's notifications.
type Handler struct {
	db *pgxpool.Pool
}

// New constructs the notifications module.
func New(db *pgxpool.Pool) *Handler {
	return &Handler{db: db}
}

// RegisterRoutes mounts notification routes on an authenticated router rooted at /v1.
func RegisterRoutes(router fiber.Router, db *pgxpool.Pool) {
	New(db).RegisterRoutes(router)
}

// RegisterRoutes mounts routes. The supplied router must already use authentication middleware.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/notifications", h.list)
	router.Put("/notifications/read-all", h.readAll)
	router.Put("/notifications/:notificationId/read", h.readOne)
}

type notification struct {
	ID        uuid.UUID       `json:"id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	ReadAt    *time.Time      `json:"read_at,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

type listResponse struct {
	Items       []notification `json:"items"`
	UnreadCount int64          `json:"unread_count"`
	HasMore     bool           `json:"has_more"`
}

type readAllResponse struct {
	UpdatedCount int64 `json:"updated_count"`
	UnreadCount  int64 `json:"unread_count"`
}

func (h *Handler) list(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return authenticationProblem(c)
	}
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", "Limit must be an integer between 1 and 100.")
	}

	var unreadCount int64
	if err := h.db.QueryRow(c.UserContext(), `
		SELECT count(*)
		FROM notifications
		WHERE user_id = $1 AND read_at IS NULL`, userID).Scan(&unreadCount); err != nil {
		return internalProblem(c)
	}

	rows, err := h.db.Query(c.UserContext(), `
		SELECT id, notification_type, payload, read_at, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2`, userID, limit+1)
	if err != nil {
		return internalProblem(c)
	}
	defer rows.Close()

	items := make([]notification, 0, limit)
	for rows.Next() {
		item, err := scanNotification(rows)
		if err != nil {
			return internalProblem(c)
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return internalProblem(c)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return httpx.OK(c, listResponse{Items: items, UnreadCount: unreadCount, HasMore: hasMore})
}

func (h *Handler) readOne(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return authenticationProblem(c)
	}
	notificationID, err := httpx.UUIDParam(c, "notificationId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_notification_id", "Notification ID must be a valid UUID.")
	}

	item, err := scanNotification(h.db.QueryRow(c.UserContext(), `
		UPDATE notifications
		SET read_at = COALESCE(read_at, now())
		WHERE id = $1 AND user_id = $2
		RETURNING id, notification_type, payload, read_at, created_at`, notificationID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.Problem(c, fiber.StatusNotFound, "notification_not_found", "The notification was not found.")
	}
	if err != nil {
		return internalProblem(c)
	}
	return httpx.OK(c, item)
}

func (h *Handler) readAll(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return authenticationProblem(c)
	}
	command, err := h.db.Exec(c.UserContext(), `
		UPDATE notifications
		SET read_at = now()
		WHERE user_id = $1 AND read_at IS NULL`, userID)
	if err != nil {
		return internalProblem(c)
	}
	return httpx.OK(c, readAllResponse{UpdatedCount: command.RowsAffected(), UnreadCount: 0})
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNotification(row rowScanner) (notification, error) {
	var result notification
	var payload []byte
	if err := row.Scan(&result.ID, &result.Type, &payload, &result.ReadAt, &result.CreatedAt); err != nil {
		return notification{}, err
	}
	if len(payload) == 0 || !json.Valid(payload) {
		payload = []byte(`{}`)
	}
	result.Payload = json.RawMessage(payload)
	return result, nil
}

func parseLimit(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultLimit, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > maximumLimit {
		return 0, errors.New("invalid limit")
	}
	return limit, nil
}

func authenticationProblem(c *fiber.Ctx) error {
	return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
}

func internalProblem(c *fiber.Ctx) error {
	return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "The request could not be completed.")
}
