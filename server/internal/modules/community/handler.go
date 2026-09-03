package community

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"petconnect/server/internal/platform/httpx"
)

type Handler struct {
	db *pgxpool.Pool
}

type Community struct {
	ID          uuid.UUID `json:"id"`
	OwnerUserID uuid.UUID `json:"owner_user_id"`
	OwnerName   string    `json:"owner_name"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Emoji       string    `json:"emoji"`
	MemberCount int64     `json:"member_count"`
	JoinedByMe  bool      `json:"joined_by_me"`
	MyRole      string    `json:"my_role,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type communityPage struct {
	Items      []Community `json:"items"`
	NextCursor string      `json:"next_cursor,omitempty"`
}

type createRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Emoji       string `json:"emoji"`
}

type cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func (h *Handler) list(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", err.Error())
	}
	var cursorTime any
	var cursorID any
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		value, decodeErr := decodeCursor(raw)
		if decodeErr != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_cursor", "The pagination cursor is invalid.")
		}
		cursorTime, cursorID = value.CreatedAt, value.ID
	}
	query := strings.TrimSpace(c.Query("q"))
	rows, err := h.db.Query(c.UserContext(), `
		SELECT cm.id, cm.owner_user_id, owner.name, cm.name, cm.description, cm.emoji,
		       (SELECT count(*) FROM community_members member WHERE member.community_id = cm.id),
		       membership.user_id IS NOT NULL,
		       COALESCE(membership.role, ''), cm.created_at
		FROM communities cm
		JOIN users owner ON owner.id = cm.owner_user_id
		LEFT JOIN community_members membership ON membership.community_id = cm.id AND membership.user_id = $1
		WHERE ($2 = '' OR cm.name ILIKE '%' || $2 || '%' OR cm.description ILIKE '%' || $2 || '%')
		  AND ($3::timestamptz IS NULL OR (cm.created_at, cm.id) < ($3::timestamptz, $4::uuid))
		ORDER BY cm.created_at DESC, cm.id DESC
		LIMIT $5`, userID, query, cursorTime, cursorID, limit+1)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "community_query_failed", "Communities could not be loaded.")
	}
	defer rows.Close()
	items := make([]Community, 0, limit)
	for rows.Next() {
		var item Community
		if err := rows.Scan(&item.ID, &item.OwnerUserID, &item.OwnerName, &item.Name, &item.Description, &item.Emoji, &item.MemberCount, &item.JoinedByMe, &item.MyRole, &item.CreatedAt); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "community_query_failed", "Communities could not be loaded.")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "community_query_failed", "Communities could not be loaded.")
	}
	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
	}
	return httpx.OK(c, communityPage{Items: items, NextCursor: nextCursor})
}

func (h *Handler) create(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	var request createRequest
	if err := c.BodyParser(&request); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body must be valid JSON.")
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	request.Emoji = strings.TrimSpace(request.Emoji)
	if request.Emoji == "" {
		request.Emoji = "🐾"
	}
	if length := utf8.RuneCountInString(request.Name); length < 2 || length > 80 {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_name", "name must contain between 2 and 80 characters.")
	}
	if utf8.RuneCountInString(request.Description) > 2000 {
		return httpx.Problem(c, fiber.StatusBadRequest, "description_too_long", "description must contain at most 2000 characters.")
	}
	if utf8.RuneCountInString(request.Emoji) > 16 {
		return httpx.Problem(c, fiber.StatusBadRequest, "emoji_too_long", "emoji must contain at most 16 characters.")
	}

	tx, err := h.db.Begin(c.UserContext())
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "community_create_failed", "The community could not be created.")
	}
	defer func() { _ = tx.Rollback(c.UserContext()) }()

	var item Community
	err = tx.QueryRow(c.UserContext(), `
		INSERT INTO communities (owner_user_id, name, description, emoji)
		VALUES ($1, $2, $3, $4)
		RETURNING id, owner_user_id, name, description, emoji, created_at`, userID, request.Name, request.Description, request.Emoji).Scan(
		&item.ID, &item.OwnerUserID, &item.Name, &item.Description, &item.Emoji, &item.CreatedAt,
	)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" {
			return httpx.Problem(c, fiber.StatusConflict, "community_name_taken", "A community with this name already exists.")
		}
		return httpx.Problem(c, fiber.StatusInternalServerError, "community_create_failed", "The community could not be created.")
	}
	if _, err := tx.Exec(c.UserContext(), `INSERT INTO community_members (community_id, user_id, role) VALUES ($1, $2, 'owner')`, item.ID, userID); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "community_create_failed", "The community could not be created.")
	}
	if err := tx.QueryRow(c.UserContext(), "SELECT name FROM users WHERE id = $1", userID).Scan(&item.OwnerName); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "community_create_failed", "The community could not be created.")
	}
	if err := tx.Commit(c.UserContext()); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "community_create_failed", "The community could not be created.")
	}
	item.MemberCount = 1
	item.JoinedByMe = true
	item.MyRole = "owner"
	return httpx.Created(c, item)
}

func (h *Handler) join(c *fiber.Ctx) error {
	userID, communityID, err := membershipIDs(c)
	if err != nil {
		return err
	}
	result, err := h.db.Exec(c.UserContext(), `
		INSERT INTO community_members (community_id, user_id, role)
		SELECT id, $2, 'member' FROM communities WHERE id = $1
		ON CONFLICT DO NOTHING`, communityID, userID)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "community_join_failed", "The community could not be joined.")
	}
	if result.RowsAffected() == 0 {
		var exists bool
		if err := h.db.QueryRow(c.UserContext(), "SELECT EXISTS (SELECT 1 FROM communities WHERE id = $1)", communityID).Scan(&exists); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "community_join_failed", "The community could not be joined.")
		}
		if !exists {
			return httpx.Problem(c, fiber.StatusNotFound, "community_not_found", "The requested community was not found.")
		}
	}
	return httpx.OK(c, fiber.Map{"joined": true})
}

func (h *Handler) leave(c *fiber.Ctx) error {
	userID, communityID, err := membershipIDs(c)
	if err != nil {
		return err
	}
	var exists bool
	var role string
	err = h.db.QueryRow(c.UserContext(), `
		SELECT EXISTS (SELECT 1 FROM communities WHERE id = $1),
		       COALESCE((SELECT role FROM community_members WHERE community_id = $1 AND user_id = $2), '')`, communityID, userID).Scan(&exists, &role)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "community_leave_failed", "The community could not be left.")
	}
	if !exists {
		return httpx.Problem(c, fiber.StatusNotFound, "community_not_found", "The requested community was not found.")
	}
	if role == "owner" {
		return httpx.Problem(c, fiber.StatusConflict, "owner_cannot_leave", "Transfer or delete the community before its owner can leave.")
	}
	if _, err := h.db.Exec(c.UserContext(), "DELETE FROM community_members WHERE community_id = $1 AND user_id = $2", communityID, userID); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "community_leave_failed", "The community could not be left.")
	}
	return httpx.OK(c, fiber.Map{"joined": false})
}

func membershipIDs(c *fiber.Ctx) (uuid.UUID, uuid.UUID, error) {
	userID, err := httpx.UserID(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	communityID, err := httpx.UUIDParam(c, "communityId")
	if err != nil {
		return uuid.Nil, uuid.Nil, httpx.Problem(c, fiber.StatusBadRequest, "invalid_community_id", "communityId must be a valid UUID.")
	}
	return userID, communityID, nil
}

func parseLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 20, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 50 {
		return 0, fmt.Errorf("limit must be between 1 and 50")
	}
	return value, nil
}

func encodeCursor(createdAt time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(createdAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()))
}

func decodeCursor(raw string) (cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursor{}, err
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return cursor{}, fmt.Errorf("invalid cursor")
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return cursor{}, err
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return cursor{}, err
	}
	return cursor{CreatedAt: createdAt, ID: id}, nil
}
