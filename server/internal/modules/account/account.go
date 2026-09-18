package account

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"petconnect/server/internal/platform/httpx"
)

// Handler owns the authenticated owner's profile.
type Handler struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

// New constructs the account module. redisClient is used only by account
// deletion, to invalidate already-issued access tokens near-immediately —
// the same mechanism logout-all uses (ADR 0004) — since deleting an
// account is at least as security-sensitive as signing out everywhere.
func New(db *pgxpool.Pool, redisClient *redis.Client) *Handler {
	return &Handler{db: db, redis: redisClient}
}

// RegisterRoutes mounts authenticated account routes on a router rooted at /v1.
func RegisterRoutes(router fiber.Router, db *pgxpool.Pool, redisClient *redis.Client) {
	New(db, redisClient).RegisterRoutes(router)
}

// RegisterRoutes mounts routes. The supplied router must already use authentication middleware.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/me", h.getMe)
	router.Patch("/me", h.patchMe)
	router.Delete("/me/account", h.deleteAccount)
	router.Get("/me/export", h.exportData)
}

type location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type user struct {
	ID                    uuid.UUID  `json:"id"`
	Email                 string     `json:"email"`
	Name                  string     `json:"name"`
	Bio                   string     `json:"bio"`
	City                  string     `json:"city"`
	ProfilePhotoURL       string     `json:"profile_photo_url"`
	Location              *location  `json:"location,omitempty"`
	IsPrivate             bool       `json:"is_private"`
	OnboardingCompleted   bool       `json:"onboarding_completed"`
	OnboardingCompletedAt *time.Time `json:"onboarding_completed_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type patchRequest struct {
	Name               *string   `json:"name"`
	Bio                *string   `json:"bio"`
	City               *string   `json:"city"`
	ProfilePhotoURL    *string   `json:"profile_photo_url"`
	Location           *location `json:"location"`
	ClearLocation      bool      `json:"clear_location"`
	IsPrivate          *bool     `json:"is_private"`
	CompleteOnboarding bool      `json:"complete_onboarding"`
}

func (h *Handler) getMe(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}

	result, err := loadUser(c, h.db, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.Problem(c, fiber.StatusNotFound, "account_not_found", "The account no longer exists.")
	}
	if err != nil {
		return internalProblem(c)
	}
	return httpx.OK(c, result)
}

func (h *Handler) patchMe(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}

	var input patchRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	if input.Location != nil && input.ClearLocation {
		return validationProblem(c, "location", "Location and clear_location cannot be supplied together.")
	}
	if !hasPatch(input) {
		return validationProblem(c, "body", "Supply at least one profile field to update.")
	}
	if err := normalizeAndValidate(&input); err != nil {
		return validationProblem(c, err.field, err.message)
	}

	var latitude, longitude any
	setLocation := input.Location != nil
	if setLocation {
		latitude = input.Location.Latitude
		longitude = input.Location.Longitude
	}

	row := h.db.QueryRow(c.UserContext(), `
		UPDATE users
		SET name = COALESCE($2, name),
		    bio = COALESCE($3, bio),
		    city = COALESCE($4, city),
		    profile_photo_url = COALESCE($5, profile_photo_url),
		    location = CASE
		      WHEN $6::boolean THEN ST_SetSRID(ST_MakePoint($8::float8, $7::float8), 4326)::geography
		      WHEN $9::boolean THEN NULL
		      ELSE location
		    END,
		    onboarding_completed_at = CASE
		      WHEN $10::boolean THEN COALESCE(onboarding_completed_at, now())
		      ELSE onboarding_completed_at
		    END,
		    is_private = COALESCE($11, is_private)
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, email::text, name, bio, city, profile_photo_url,
		          ST_Y(location::geometry), ST_X(location::geometry),
		          onboarding_completed_at IS NOT NULL, onboarding_completed_at,
		          created_at, updated_at, is_private`,
		userID, input.Name, input.Bio, input.City, input.ProfilePhotoURL,
		setLocation, latitude, longitude, input.ClearLocation, input.CompleteOnboarding,
		input.IsPrivate)

	result, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.Problem(c, fiber.StatusNotFound, "account_not_found", "The account no longer exists.")
	}
	if err != nil {
		return internalProblem(c)
	}
	return httpx.OK(c, result)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func loadUser(c *fiber.Ctx, db *pgxpool.Pool, userID uuid.UUID) (user, error) {
	return scanUser(db.QueryRow(c.UserContext(), `
		SELECT id, email::text, name, bio, city, profile_photo_url,
		       ST_Y(location::geometry), ST_X(location::geometry),
		       onboarding_completed_at IS NOT NULL, onboarding_completed_at,
		       created_at, updated_at, is_private
		FROM users
		WHERE id = $1 AND deleted_at IS NULL`, userID))
}

func scanUser(row rowScanner) (user, error) {
	var result user
	var latitude, longitude *float64
	err := row.Scan(
		&result.ID, &result.Email, &result.Name, &result.Bio, &result.City,
		&result.ProfilePhotoURL, &latitude, &longitude,
		&result.OnboardingCompleted, &result.OnboardingCompletedAt,
		&result.CreatedAt, &result.UpdatedAt, &result.IsPrivate,
	)
	if err != nil {
		return user{}, err
	}
	if latitude != nil && longitude != nil {
		result.Location = &location{Latitude: *latitude, Longitude: *longitude}
	}
	return result, nil
}

type fieldError struct {
	field   string
	message string
}

func (e *fieldError) Error() string { return e.message }

func hasPatch(input patchRequest) bool {
	return input.Name != nil || input.Bio != nil || input.City != nil ||
		input.ProfilePhotoURL != nil || input.Location != nil || input.ClearLocation ||
		input.IsPrivate != nil || input.CompleteOnboarding
}

func normalizeAndValidate(input *patchRequest) *fieldError {
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		if value == "" || len([]rune(value)) > 100 {
			return &fieldError{"name", "Name must contain between 1 and 100 characters."}
		}
		input.Name = &value
	}
	if input.Bio != nil {
		value := strings.TrimSpace(*input.Bio)
		if len([]rune(value)) > 500 {
			return &fieldError{"bio", "Bio cannot exceed 500 characters."}
		}
		input.Bio = &value
	}
	if input.City != nil {
		value := strings.TrimSpace(*input.City)
		if len([]rune(value)) > 120 {
			return &fieldError{"city", "City cannot exceed 120 characters."}
		}
		input.City = &value
	}
	if input.ProfilePhotoURL != nil {
		value := strings.TrimSpace(*input.ProfilePhotoURL)
		if !validMediaURL(value) {
			return &fieldError{"profile_photo_url", "Profile photo must be an HTTP URL or a server media path."}
		}
		input.ProfilePhotoURL = &value
	}
	if input.Location != nil && !validCoordinates(input.Location.Latitude, input.Location.Longitude) {
		return &fieldError{"location", "Latitude must be between -90 and 90 and longitude between -180 and 180."}
	}
	return nil
}

func validCoordinates(latitude, longitude float64) bool {
	return latitude >= -90 && latitude <= 90 && longitude >= -180 && longitude <= 180
}

func validMediaURL(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 2048 {
		return false
	}
	if strings.HasPrefix(value, "/media/") {
		return true
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
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
