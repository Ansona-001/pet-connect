package adoption

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"petconnect/server/internal/platform/httpx"
)

type Handler struct {
	db *pgxpool.Pool
}

type Listing struct {
	ID          uuid.UUID `json:"id"`
	OwnerUserID uuid.UUID `json:"owner_user_id"`
	OwnerName   string    `json:"owner_name"`
	Name        string    `json:"name"`
	PetType     string    `json:"pet_type"`
	Breed       string    `json:"breed"`
	AgeLabel    string    `json:"age_label"`
	Gender      string    `json:"gender"`
	Description string    `json:"description"`
	City        string    `json:"city"`
	ImageURL    string    `json:"image_url"`
	Status      string    `json:"status"`
	SavedByMe   bool      `json:"saved_by_me"`
	SaveCount   int64     `json:"save_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type listingPage struct {
	Items      []Listing `json:"items"`
	NextCursor string    `json:"next_cursor,omitempty"`
}

type createRequest struct {
	Name        string `json:"name"`
	PetType     string `json:"pet_type"`
	Breed       string `json:"breed"`
	AgeLabel    string `json:"age_label"`
	Gender      string `json:"gender"`
	Description string `json:"description"`
	City        string `json:"city"`
	ImageURL    string `json:"image_url"`
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
	petType := strings.ToLower(strings.TrimSpace(c.Query("type")))
	breed := strings.TrimSpace(c.Query("breed"))
	city := strings.TrimSpace(c.Query("city"))
	gender := strings.ToLower(strings.TrimSpace(c.Query("gender")))
	query := strings.TrimSpace(c.Query("q"))
	if gender != "" && gender != "male" && gender != "female" && gender != "unknown" {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_gender", "gender must be male, female, or unknown.")
	}

	rows, err := h.db.Query(c.UserContext(), `
		SELECT l.id, l.owner_user_id, owner.name, l.name, l.pet_type, l.breed,
		       l.age_label, l.gender, l.description, l.city, l.image_url, l.status,
		       EXISTS (SELECT 1 FROM adoption_saves s WHERE s.listing_id = l.id AND s.user_id = $1),
		       (SELECT count(*) FROM adoption_saves s WHERE s.listing_id = l.id),
		       l.created_at, l.updated_at
		FROM adoption_listings l
		JOIN users owner ON owner.id = l.owner_user_id
		WHERE l.status IN ('available', 'pending')
		  AND owner.deleted_at IS NULL
		  AND ($2 = '' OR lower(l.pet_type) = $2)
		  AND ($3 = '' OR l.breed ILIKE '%' || $3 || '%')
		  AND ($4 = '' OR l.city ILIKE '%' || $4 || '%')
		  AND ($5 = '' OR lower(l.gender) = $5)
		  AND ($6 = '' OR l.name ILIKE '%' || $6 || '%' OR l.breed ILIKE '%' || $6 || '%' OR l.description ILIKE '%' || $6 || '%')
		  AND ($7::timestamptz IS NULL OR (l.created_at, l.id) < ($7::timestamptz, $8::uuid))
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT $9`, userID, petType, breed, city, gender, query, cursorTime, cursorID, limit+1)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "adoption_query_failed", "Adoption listings could not be loaded.")
	}
	defer rows.Close()
	items := make([]Listing, 0, limit)
	for rows.Next() {
		var item Listing
		if err := rows.Scan(
			&item.ID, &item.OwnerUserID, &item.OwnerName, &item.Name, &item.PetType,
			&item.Breed, &item.AgeLabel, &item.Gender, &item.Description, &item.City,
			&item.ImageURL, &item.Status, &item.SavedByMe, &item.SaveCount,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "adoption_query_failed", "Adoption listings could not be loaded.")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "adoption_query_failed", "Adoption listings could not be loaded.")
	}
	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		last := items[len(items)-1]
		nextCursor = encodeCursor(last.CreatedAt, last.ID)
	}
	return httpx.OK(c, listingPage{Items: items, NextCursor: nextCursor})
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
	request.PetType = strings.ToLower(strings.TrimSpace(request.PetType))
	request.Breed = strings.TrimSpace(request.Breed)
	request.AgeLabel = strings.TrimSpace(request.AgeLabel)
	request.Gender = strings.ToLower(strings.TrimSpace(request.Gender))
	request.Description = strings.TrimSpace(request.Description)
	request.City = strings.TrimSpace(request.City)
	request.ImageURL = strings.TrimSpace(request.ImageURL)
	if length := utf8.RuneCountInString(request.Name); length < 1 || length > 80 {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_name", "name must contain between 1 and 80 characters.")
	}
	if length := utf8.RuneCountInString(request.PetType); length < 2 || length > 40 {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_pet_type", "pet_type must contain between 2 and 40 characters.")
	}
	if utf8.RuneCountInString(request.Breed) > 100 || utf8.RuneCountInString(request.AgeLabel) > 60 {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_listing_details", "breed or age_label is too long.")
	}
	if request.Gender == "" {
		request.Gender = "unknown"
	}
	if request.Gender != "male" && request.Gender != "female" && request.Gender != "unknown" {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_gender", "gender must be male, female, or unknown.")
	}
	if utf8.RuneCountInString(request.Description) > 5000 {
		return httpx.Problem(c, fiber.StatusBadRequest, "description_too_long", "description must contain at most 5000 characters.")
	}
	if length := utf8.RuneCountInString(request.City); length < 2 || length > 100 {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_city", "city must contain between 2 and 100 characters.")
	}
	if request.ImageURL != "" && !validURL(request.ImageURL) {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_image_url", "image_url must be an HTTP(S) URL or an application-relative media path.")
	}

	var listingID uuid.UUID
	err = h.db.QueryRow(c.UserContext(), `
		INSERT INTO adoption_listings (owner_user_id, name, pet_type, breed, age_label, gender, description, city, image_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`, userID, request.Name, request.PetType, request.Breed, request.AgeLabel, request.Gender, request.Description, request.City, request.ImageURL).Scan(&listingID)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "adoption_create_failed", "The adoption listing could not be created.")
	}
	item, err := h.byID(c, userID, listingID)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "adoption_create_failed", "The listing was created but could not be loaded.")
	}
	return httpx.Created(c, item)
}

func (h *Handler) byID(c *fiber.Ctx, userID, listingID uuid.UUID) (Listing, error) {
	var item Listing
	err := h.db.QueryRow(c.UserContext(), `
		SELECT l.id, l.owner_user_id, owner.name, l.name, l.pet_type, l.breed,
		       l.age_label, l.gender, l.description, l.city, l.image_url, l.status,
		       EXISTS (SELECT 1 FROM adoption_saves s WHERE s.listing_id = l.id AND s.user_id = $1),
		       (SELECT count(*) FROM adoption_saves s WHERE s.listing_id = l.id),
		       l.created_at, l.updated_at
		FROM adoption_listings l JOIN users owner ON owner.id = l.owner_user_id
		WHERE l.id = $2`, userID, listingID).Scan(
		&item.ID, &item.OwnerUserID, &item.OwnerName, &item.Name, &item.PetType,
		&item.Breed, &item.AgeLabel, &item.Gender, &item.Description, &item.City,
		&item.ImageURL, &item.Status, &item.SavedByMe, &item.SaveCount,
		&item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}

func (h *Handler) save(c *fiber.Ctx) error {
	return h.setSaved(c, true)
}

func (h *Handler) unsave(c *fiber.Ctx) error {
	return h.setSaved(c, false)
}

func (h *Handler) setSaved(c *fiber.Ctx, saved bool) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	listingID, err := httpx.UUIDParam(c, "listingId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_listing_id", "listingId must be a valid UUID.")
	}
	var exists bool
	if err := h.db.QueryRow(c.UserContext(), "SELECT EXISTS (SELECT 1 FROM adoption_listings WHERE id = $1)", listingID).Scan(&exists); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "adoption_save_failed", "The saved state could not be updated.")
	}
	if !exists {
		return httpx.Problem(c, fiber.StatusNotFound, "listing_not_found", "The requested adoption listing was not found.")
	}
	if saved {
		_, err = h.db.Exec(c.UserContext(), "INSERT INTO adoption_saves (listing_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", listingID, userID)
	} else {
		_, err = h.db.Exec(c.UserContext(), "DELETE FROM adoption_saves WHERE listing_id = $1 AND user_id = $2", listingID, userID)
	}
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "adoption_save_failed", "The saved state could not be updated.")
	}
	return httpx.OK(c, fiber.Map{"saved": saved})
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

func validURL(value string) bool {
	if value == "" || len(value) > 4096 {
		return false
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return true
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.Host != "" && (parsed.Scheme == "https" || parsed.Scheme == "http")
}
