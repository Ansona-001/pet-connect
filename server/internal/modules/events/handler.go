package events

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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"petconnect/server/internal/platform/httpx"
)

type Handler struct {
	db *pgxpool.Pool
}

type Event struct {
	ID            uuid.UUID  `json:"id"`
	CreatorUserID uuid.UUID  `json:"creator_user_id"`
	CreatorName   string     `json:"creator_name"`
	CommunityID   *uuid.UUID `json:"community_id,omitempty"`
	CommunityName string     `json:"community_name,omitempty"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	LocationName  string     `json:"location_name"`
	Latitude      *float64   `json:"latitude,omitempty"`
	Longitude     *float64   `json:"longitude,omitempty"`
	StartsAt      time.Time  `json:"starts_at"`
	AttendeeCount int64      `json:"attendee_count"`
	MyRSVP        string     `json:"my_rsvp,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type eventPage struct {
	Items      []Event `json:"items"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

type createRequest struct {
	CommunityID  string   `json:"community_id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	LocationName string   `json:"location_name"`
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`
	StartsAt     string   `json:"starts_at"`
}

type rsvpRequest struct {
	Status string `json:"status"`
}

type cursor struct {
	StartsAt time.Time
	ID       uuid.UUID
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
		cursorTime, cursorID = value.StartsAt, value.ID
	}
	var communityID any
	if raw := strings.TrimSpace(c.Query("community_id")); raw != "" {
		parsed, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_community_id", "community_id must be a valid UUID.")
		}
		communityID = parsed
	}
	query := strings.TrimSpace(c.Query("q"))
	includePast := strings.EqualFold(strings.TrimSpace(c.Query("include_past")), "true")

	rows, err := h.db.Query(c.UserContext(), `
		SELECT e.id, e.creator_user_id, creator.name, e.community_id,
		       COALESCE(cm.name, ''), e.title, e.description, e.location_name,
		       e.location IS NOT NULL,
		       COALESCE(ST_Y(e.location::geometry), 0), COALESCE(ST_X(e.location::geometry), 0),
		       e.starts_at,
		       (SELECT count(*) FROM event_rsvps r WHERE r.event_id = e.id AND r.status = 'going'),
		       COALESCE((SELECT r.status FROM event_rsvps r WHERE r.event_id = e.id AND r.user_id = $1), ''),
		       e.created_at
		FROM events e
		JOIN users creator ON creator.id = e.creator_user_id
		LEFT JOIN communities cm ON cm.id = e.community_id
		WHERE ($2::uuid IS NULL OR e.community_id = $2::uuid)
		  AND ($3 = '' OR e.title ILIKE '%' || $3 || '%' OR e.description ILIKE '%' || $3 || '%' OR e.location_name ILIKE '%' || $3 || '%')
		  AND ($4::boolean OR e.starts_at >= now())
		  AND ($5::timestamptz IS NULL OR (e.starts_at, e.id) > ($5::timestamptz, $6::uuid))
		ORDER BY e.starts_at ASC, e.id ASC
		LIMIT $7`, userID, communityID, query, includePast, cursorTime, cursorID, limit+1)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "event_query_failed", "Events could not be loaded.")
	}
	defer rows.Close()
	items := make([]Event, 0, limit)
	for rows.Next() {
		var item Event
		var hasLocation bool
		var latitude, longitude float64
		if err := rows.Scan(
			&item.ID, &item.CreatorUserID, &item.CreatorName, &item.CommunityID,
			&item.CommunityName, &item.Title, &item.Description, &item.LocationName,
			&hasLocation, &latitude, &longitude, &item.StartsAt, &item.AttendeeCount,
			&item.MyRSVP, &item.CreatedAt,
		); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "event_query_failed", "Events could not be loaded.")
		}
		if hasLocation {
			item.Latitude, item.Longitude = &latitude, &longitude
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "event_query_failed", "Events could not be loaded.")
	}
	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeCursor(last.StartsAt, last.ID)
	}
	return httpx.OK(c, eventPage{Items: items, NextCursor: nextCursor})
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
	request.Title = strings.TrimSpace(request.Title)
	request.Description = strings.TrimSpace(request.Description)
	request.LocationName = strings.TrimSpace(request.LocationName)
	if length := utf8.RuneCountInString(request.Title); length < 2 || length > 160 {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_title", "title must contain between 2 and 160 characters.")
	}
	if utf8.RuneCountInString(request.Description) > 5000 {
		return httpx.Problem(c, fiber.StatusBadRequest, "description_too_long", "description must contain at most 5000 characters.")
	}
	if length := utf8.RuneCountInString(request.LocationName); length < 2 || length > 200 {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_location_name", "location_name must contain between 2 and 200 characters.")
	}
	startsAt, err := time.Parse(time.RFC3339, strings.TrimSpace(request.StartsAt))
	if err != nil || !startsAt.After(time.Now()) {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_starts_at", "starts_at must be a future RFC3339 timestamp.")
	}
	if (request.Latitude == nil) != (request.Longitude == nil) {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_location", "latitude and longitude must be supplied together.")
	}
	if request.Latitude != nil && (*request.Latitude < -90 || *request.Latitude > 90) {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_latitude", "latitude must be between -90 and 90.")
	}
	if request.Longitude != nil && (*request.Longitude < -180 || *request.Longitude > 180) {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_longitude", "longitude must be between -180 and 180.")
	}

	var communityID any
	if strings.TrimSpace(request.CommunityID) != "" {
		parsed, parseErr := uuid.Parse(strings.TrimSpace(request.CommunityID))
		if parseErr != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_community_id", "community_id must be a valid UUID.")
		}
		var exists, member bool
		err = h.db.QueryRow(c.UserContext(), `
			SELECT EXISTS (SELECT 1 FROM communities WHERE id = $1),
			       EXISTS (SELECT 1 FROM community_members WHERE community_id = $1 AND user_id = $2)`, parsed, userID).Scan(&exists, &member)
		if err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "event_create_failed", "The event could not be created.")
		}
		if !exists {
			return httpx.Problem(c, fiber.StatusNotFound, "community_not_found", "The selected community was not found.")
		}
		if !member {
			return httpx.Problem(c, fiber.StatusForbidden, "community_membership_required", "Join the community before creating an event for it.")
		}
		communityID = parsed
	}

	var eventID uuid.UUID
	err = h.db.QueryRow(c.UserContext(), `
		INSERT INTO events (creator_user_id, community_id, title, description, location_name, location, starts_at)
		VALUES ($1, $2::uuid, $3, $4, $5,
		  CASE WHEN $6::double precision IS NULL THEN NULL
		       ELSE ST_SetSRID(ST_MakePoint($7::double precision, $6::double precision), 4326)::geography END,
		  $8)
		RETURNING id`, userID, communityID, request.Title, request.Description, request.LocationName, request.Latitude, request.Longitude, startsAt).Scan(&eventID)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "event_create_failed", "The event could not be created.")
	}
	item, err := h.byID(c, userID, eventID)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "event_create_failed", "The event was created but could not be loaded.")
	}
	return httpx.Created(c, item)
}

func (h *Handler) byID(c *fiber.Ctx, userID, eventID uuid.UUID) (Event, error) {
	var item Event
	var hasLocation bool
	var latitude, longitude float64
	err := h.db.QueryRow(c.UserContext(), `
		SELECT e.id, e.creator_user_id, creator.name, e.community_id,
		       COALESCE(cm.name, ''), e.title, e.description, e.location_name,
		       e.location IS NOT NULL,
		       COALESCE(ST_Y(e.location::geometry), 0), COALESCE(ST_X(e.location::geometry), 0),
		       e.starts_at,
		       (SELECT count(*) FROM event_rsvps r WHERE r.event_id = e.id AND r.status = 'going'),
		       COALESCE((SELECT r.status FROM event_rsvps r WHERE r.event_id = e.id AND r.user_id = $1), ''),
		       e.created_at
		FROM events e JOIN users creator ON creator.id = e.creator_user_id
		LEFT JOIN communities cm ON cm.id = e.community_id
		WHERE e.id = $2`, userID, eventID).Scan(
		&item.ID, &item.CreatorUserID, &item.CreatorName, &item.CommunityID,
		&item.CommunityName, &item.Title, &item.Description, &item.LocationName,
		&hasLocation, &latitude, &longitude, &item.StartsAt, &item.AttendeeCount,
		&item.MyRSVP, &item.CreatedAt,
	)
	if hasLocation {
		item.Latitude, item.Longitude = &latitude, &longitude
	}
	return item, err
}

func (h *Handler) setRSVP(c *fiber.Ctx) error {
	userID, eventID, err := ids(c)
	if err != nil {
		return err
	}
	var request rsvpRequest
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&request); err != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body must be valid JSON.")
		}
	}
	request.Status = strings.ToLower(strings.TrimSpace(request.Status))
	if request.Status == "" {
		request.Status = "going"
	}
	if request.Status != "going" && request.Status != "interested" {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_rsvp_status", "status must be going or interested.")
	}
	var stored string
	err = h.db.QueryRow(c.UserContext(), `
		INSERT INTO event_rsvps (event_id, user_id, status)
		SELECT id, $2, $3 FROM events WHERE id = $1
		ON CONFLICT (event_id, user_id) DO UPDATE SET status = EXCLUDED.status
		RETURNING status`, eventID, userID, request.Status).Scan(&stored)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.Problem(c, fiber.StatusNotFound, "event_not_found", "The requested event was not found.")
	}
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "event_rsvp_failed", "The RSVP could not be updated.")
	}
	return httpx.OK(c, fiber.Map{"status": stored})
}

func (h *Handler) removeRSVP(c *fiber.Ctx) error {
	userID, eventID, err := ids(c)
	if err != nil {
		return err
	}
	var exists bool
	if err = h.db.QueryRow(c.UserContext(), "SELECT EXISTS (SELECT 1 FROM events WHERE id = $1)", eventID).Scan(&exists); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "event_rsvp_failed", "The RSVP could not be removed.")
	}
	if !exists {
		return httpx.Problem(c, fiber.StatusNotFound, "event_not_found", "The requested event was not found.")
	}
	if _, err := h.db.Exec(c.UserContext(), "DELETE FROM event_rsvps WHERE event_id = $1 AND user_id = $2", eventID, userID); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "event_rsvp_failed", "The RSVP could not be removed.")
	}
	return httpx.OK(c, fiber.Map{"status": nil})
}

func ids(c *fiber.Ctx) (uuid.UUID, uuid.UUID, error) {
	userID, err := httpx.UserID(c)
	if err != nil {
		return uuid.Nil, uuid.Nil, httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	eventID, err := httpx.UUIDParam(c, "eventId")
	if err != nil {
		return uuid.Nil, uuid.Nil, httpx.Problem(c, fiber.StatusBadRequest, "invalid_event_id", "eventId must be a valid UUID.")
	}
	return userID, eventID, nil
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

func encodeCursor(startsAt time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(startsAt.UTC().Format(time.RFC3339Nano) + "|" + id.String()))
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
	startsAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return cursor{}, err
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return cursor{}, err
	}
	return cursor{StartsAt: startsAt, ID: id}, nil
}
