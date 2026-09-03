package pets

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"petconnect/server/internal/platform/httpx"
)

const petColumns = `id, owner_id, name, pet_type, breed, birth_date::text,
	age_label, gender, weight_kg::float8, bio, personality, interests,
	primary_image_url, ST_Y(location::geometry), ST_X(location::geometry),
	is_verified, status, created_at, updated_at`

var allowedPetTypes = map[string]struct{}{
	"dog": {}, "cat": {}, "bird": {}, "rabbit": {}, "exotic": {}, "other": {},
}

var allowedGenders = map[string]struct{}{
	"male": {}, "female": {}, "unknown": {},
}

var allowedStatuses = map[string]struct{}{
	"active": {}, "paused": {}, "adopted": {},
}

// Handler owns pet profiles and owner-scoped mutations.
type Handler struct {
	db    *pgxpool.Pool
	clock func() time.Time
}

// New constructs the pet module.
func New(db *pgxpool.Pool) *Handler {
	return &Handler{db: db, clock: time.Now}
}

// RegisterRoutes mounts pet routes on an authenticated router rooted at /v1.
func RegisterRoutes(router fiber.Router, db *pgxpool.Pool) {
	New(db).RegisterRoutes(router)
}

// RegisterRoutes mounts routes. The supplied router must already use authentication middleware.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	router.Get("/me/pets", h.listMine)
	router.Post("/me/pets", h.create)
	router.Get("/pets/:petId", h.get)
	router.Patch("/pets/:petId", h.patch)
	router.Delete("/pets/:petId", h.delete)
}

type location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type pet struct {
	ID              uuid.UUID `json:"id"`
	OwnerID         uuid.UUID `json:"owner_id"`
	Name            string    `json:"name"`
	PetType         string    `json:"pet_type"`
	Breed           string    `json:"breed"`
	BirthDate       *string   `json:"birth_date,omitempty"`
	AgeLabel        string    `json:"age_label"`
	Gender          string    `json:"gender"`
	WeightKG        *float64  `json:"weight_kg,omitempty"`
	Bio             string    `json:"bio"`
	Personality     []string  `json:"personality"`
	Interests       []string  `json:"interests"`
	PrimaryImageURL string    `json:"primary_image_url"`
	Location        *location `json:"location,omitempty"`
	IsVerified      bool      `json:"is_verified"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type createRequest struct {
	Name            string    `json:"name"`
	PetType         string    `json:"pet_type"`
	Breed           string    `json:"breed"`
	BirthDate       *string   `json:"birth_date"`
	AgeLabel        string    `json:"age_label"`
	Gender          string    `json:"gender"`
	WeightKG        *float64  `json:"weight_kg"`
	Bio             string    `json:"bio"`
	Personality     []string  `json:"personality"`
	Interests       []string  `json:"interests"`
	PrimaryImageURL string    `json:"primary_image_url"`
	Location        *location `json:"location"`
}

type patchRequest struct {
	Name            *string   `json:"name"`
	PetType         *string   `json:"pet_type"`
	Breed           *string   `json:"breed"`
	BirthDate       *string   `json:"birth_date"`
	AgeLabel        *string   `json:"age_label"`
	Gender          *string   `json:"gender"`
	WeightKG        *float64  `json:"weight_kg"`
	ClearWeight     bool      `json:"clear_weight"`
	Bio             *string   `json:"bio"`
	Personality     *[]string `json:"personality"`
	Interests       *[]string `json:"interests"`
	PrimaryImageURL *string   `json:"primary_image_url"`
	Location        *location `json:"location"`
	ClearLocation   bool      `json:"clear_location"`
	Status          *string   `json:"status"`
}

type fieldError struct {
	field   string
	message string
}

func (e *fieldError) Error() string { return e.message }

func (h *Handler) listMine(c *fiber.Ctx) error {
	ownerID, ok := authenticatedUser(c)
	if !ok {
		return authenticationProblem(c)
	}

	rows, err := h.db.Query(c.UserContext(), `
		SELECT `+petColumns+`
		FROM pets
		WHERE owner_id = $1 AND deleted_at IS NULL
		ORDER BY created_at ASC, id ASC`, ownerID)
	if err != nil {
		return internalProblem(c)
	}
	defer rows.Close()

	result := make([]pet, 0)
	for rows.Next() {
		item, err := scanPet(rows)
		if err != nil {
			return internalProblem(c)
		}
		result = append(result, item)
	}
	if rows.Err() != nil {
		return internalProblem(c)
	}
	return httpx.OK(c, result)
}

func (h *Handler) create(c *fiber.Ctx) error {
	ownerID, ok := authenticatedUser(c)
	if !ok {
		return authenticationProblem(c)
	}

	var input createRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	parsedBirthDate, validationErr := normalizeCreate(&input, h.clock().UTC())
	if validationErr != nil {
		return validationProblem(c, validationErr.field, validationErr.message)
	}

	var latitude, longitude any
	setLocation := input.Location != nil
	if setLocation {
		latitude, longitude = input.Location.Latitude, input.Location.Longitude
	}

	row := h.db.QueryRow(c.UserContext(), `
		INSERT INTO pets (
			owner_id, name, pet_type, breed, birth_date, age_label, gender,
			weight_kg, bio, personality, interests, primary_image_url, location
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			CASE WHEN $13::boolean
			  THEN ST_SetSRID(ST_MakePoint($15::float8, $14::float8), 4326)::geography
			  ELSE NULL
			END
		)
		RETURNING `+petColumns,
		ownerID, input.Name, input.PetType, input.Breed, parsedBirthDate,
		input.AgeLabel, input.Gender, input.WeightKG, input.Bio,
		input.Personality, input.Interests, input.PrimaryImageURL,
		setLocation, latitude, longitude)
	result, err := scanPet(row)
	if err != nil {
		return internalProblem(c)
	}
	return httpx.Created(c, result)
}

func (h *Handler) get(c *fiber.Ctx) error {
	requesterID, ok := authenticatedUser(c)
	if !ok {
		return authenticationProblem(c)
	}
	petID, err := parsePetID(c)
	if err != nil {
		return invalidPetIDProblem(c)
	}

	result, err := scanPet(h.db.QueryRow(c.UserContext(), `
		SELECT `+petColumns+`
		FROM pets
		WHERE id = $1 AND deleted_at IS NULL
		  AND (owner_id = $2 OR status = 'active')`, petID, requesterID))
	if errors.Is(err, pgx.ErrNoRows) {
		return notFoundProblem(c)
	}
	if err != nil {
		return internalProblem(c)
	}
	return httpx.OK(c, result)
}

func (h *Handler) patch(c *fiber.Ctx) error {
	ownerID, ok := authenticatedUser(c)
	if !ok {
		return authenticationProblem(c)
	}
	petID, err := parsePetID(c)
	if err != nil {
		return invalidPetIDProblem(c)
	}

	var input patchRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	if !hasPatch(input) {
		return validationProblem(c, "body", "Supply at least one pet field to update.")
	}
	if input.Location != nil && input.ClearLocation {
		return validationProblem(c, "location", "Location and clear_location cannot be supplied together.")
	}
	if input.WeightKG != nil && input.ClearWeight {
		return validationProblem(c, "weight_kg", "Weight and clear_weight cannot be supplied together.")
	}
	parsedBirthDate, validationErr := normalizePatch(&input, h.clock().UTC())
	if validationErr != nil {
		return validationProblem(c, validationErr.field, validationErr.message)
	}

	args := []any{petID, ownerID}
	updates := make([]string, 0, 14)
	add := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if input.Name != nil {
		updates = append(updates, "name = "+add(*input.Name))
	}
	if input.PetType != nil {
		updates = append(updates, "pet_type = "+add(*input.PetType))
	}
	if input.Breed != nil {
		updates = append(updates, "breed = "+add(*input.Breed))
	}
	if input.BirthDate != nil {
		if parsedBirthDate == nil {
			updates = append(updates, "birth_date = NULL")
		} else {
			updates = append(updates, "birth_date = "+add(*parsedBirthDate))
		}
	}
	if input.AgeLabel != nil {
		updates = append(updates, "age_label = "+add(*input.AgeLabel))
	}
	if input.Gender != nil {
		updates = append(updates, "gender = "+add(*input.Gender))
	}
	if input.ClearWeight {
		updates = append(updates, "weight_kg = NULL")
	} else if input.WeightKG != nil {
		updates = append(updates, "weight_kg = "+add(*input.WeightKG))
	}
	if input.Bio != nil {
		updates = append(updates, "bio = "+add(*input.Bio))
	}
	if input.Personality != nil {
		updates = append(updates, "personality = "+add(*input.Personality))
	}
	if input.Interests != nil {
		updates = append(updates, "interests = "+add(*input.Interests))
	}
	if input.PrimaryImageURL != nil {
		updates = append(updates, "primary_image_url = "+add(*input.PrimaryImageURL))
	}
	if input.ClearLocation {
		updates = append(updates, "location = NULL")
	} else if input.Location != nil {
		latitude := add(input.Location.Latitude)
		longitude := add(input.Location.Longitude)
		updates = append(updates, "location = ST_SetSRID(ST_MakePoint("+longitude+"::float8, "+latitude+"::float8), 4326)::geography")
	}
	if input.Status != nil {
		updates = append(updates, "status = "+add(*input.Status))
	}

	query := `UPDATE pets SET ` + strings.Join(updates, ", ") +
		` WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL RETURNING ` + petColumns
	result, err := scanPet(h.db.QueryRow(c.UserContext(), query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return notFoundProblem(c)
	}
	if err != nil {
		return internalProblem(c)
	}
	return httpx.OK(c, result)
}

func (h *Handler) delete(c *fiber.Ctx) error {
	ownerID, ok := authenticatedUser(c)
	if !ok {
		return authenticationProblem(c)
	}
	petID, err := parsePetID(c)
	if err != nil {
		return invalidPetIDProblem(c)
	}

	command, err := h.db.Exec(c.UserContext(), `
		UPDATE pets
		SET status = 'deleted', deleted_at = now()
		WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL`, petID, ownerID)
	if err != nil {
		return internalProblem(c)
	}
	if command.RowsAffected() == 0 {
		return notFoundProblem(c)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPet(row rowScanner) (pet, error) {
	var result pet
	var latitude, longitude *float64
	err := row.Scan(
		&result.ID, &result.OwnerID, &result.Name, &result.PetType, &result.Breed,
		&result.BirthDate, &result.AgeLabel, &result.Gender, &result.WeightKG,
		&result.Bio, &result.Personality, &result.Interests, &result.PrimaryImageURL,
		&latitude, &longitude, &result.IsVerified, &result.Status,
		&result.CreatedAt, &result.UpdatedAt,
	)
	if err != nil {
		return pet{}, err
	}
	if result.Personality == nil {
		result.Personality = []string{}
	}
	if result.Interests == nil {
		result.Interests = []string{}
	}
	if latitude != nil && longitude != nil {
		result.Location = &location{Latitude: *latitude, Longitude: *longitude}
	}
	return result, nil
}

func normalizeCreate(input *createRequest, now time.Time) (*time.Time, *fieldError) {
	input.Name = strings.TrimSpace(input.Name)
	input.PetType = strings.ToLower(strings.TrimSpace(input.PetType))
	input.Breed = strings.TrimSpace(input.Breed)
	input.AgeLabel = strings.TrimSpace(input.AgeLabel)
	input.Gender = strings.ToLower(strings.TrimSpace(input.Gender))
	if input.Gender == "" {
		input.Gender = "unknown"
	}
	input.Bio = strings.TrimSpace(input.Bio)
	input.PrimaryImageURL = strings.TrimSpace(input.PrimaryImageURL)

	if err := validateName(input.Name); err != nil {
		return nil, err
	}
	if err := validatePetType(input.PetType); err != nil {
		return nil, err
	}
	if len([]rune(input.Breed)) > 120 {
		return nil, &fieldError{"breed", "Breed cannot exceed 120 characters."}
	}
	if len([]rune(input.AgeLabel)) > 50 {
		return nil, &fieldError{"age_label", "Age label cannot exceed 50 characters."}
	}
	if _, ok := allowedGenders[input.Gender]; !ok {
		return nil, &fieldError{"gender", "Gender must be male, female, or unknown."}
	}
	if input.WeightKG != nil && (*input.WeightKG <= 0 || *input.WeightKG > 10000) {
		return nil, &fieldError{"weight_kg", "Weight must be greater than 0 and no more than 10000 kg."}
	}
	if len([]rune(input.Bio)) > 1000 {
		return nil, &fieldError{"bio", "Bio cannot exceed 1000 characters."}
	}
	personality, err := normalizeLabels(input.Personality, "personality")
	if err != nil {
		return nil, err
	}
	interests, err := normalizeLabels(input.Interests, "interests")
	if err != nil {
		return nil, err
	}
	input.Personality, input.Interests = personality, interests
	if !validMediaURL(input.PrimaryImageURL) {
		return nil, &fieldError{"primary_image_url", "Primary image must be an HTTP URL or a server media path."}
	}
	if input.Location != nil && !validCoordinates(input.Location.Latitude, input.Location.Longitude) {
		return nil, &fieldError{"location", "Latitude must be between -90 and 90 and longitude between -180 and 180."}
	}
	return parseBirthDate(input.BirthDate, now)
}

func normalizePatch(input *patchRequest, now time.Time) (*time.Time, *fieldError) {
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		input.Name = &value
		if err := validateName(value); err != nil {
			return nil, err
		}
	}
	if input.PetType != nil {
		value := strings.ToLower(strings.TrimSpace(*input.PetType))
		input.PetType = &value
		if err := validatePetType(value); err != nil {
			return nil, err
		}
	}
	if input.Breed != nil {
		value := strings.TrimSpace(*input.Breed)
		if len([]rune(value)) > 120 {
			return nil, &fieldError{"breed", "Breed cannot exceed 120 characters."}
		}
		input.Breed = &value
	}
	if input.AgeLabel != nil {
		value := strings.TrimSpace(*input.AgeLabel)
		if len([]rune(value)) > 50 {
			return nil, &fieldError{"age_label", "Age label cannot exceed 50 characters."}
		}
		input.AgeLabel = &value
	}
	if input.Gender != nil {
		value := strings.ToLower(strings.TrimSpace(*input.Gender))
		if _, ok := allowedGenders[value]; !ok {
			return nil, &fieldError{"gender", "Gender must be male, female, or unknown."}
		}
		input.Gender = &value
	}
	if input.WeightKG != nil && (*input.WeightKG <= 0 || *input.WeightKG > 10000) {
		return nil, &fieldError{"weight_kg", "Weight must be greater than 0 and no more than 10000 kg."}
	}
	if input.Bio != nil {
		value := strings.TrimSpace(*input.Bio)
		if len([]rune(value)) > 1000 {
			return nil, &fieldError{"bio", "Bio cannot exceed 1000 characters."}
		}
		input.Bio = &value
	}
	if input.Personality != nil {
		values, err := normalizeLabels(*input.Personality, "personality")
		if err != nil {
			return nil, err
		}
		input.Personality = &values
	}
	if input.Interests != nil {
		values, err := normalizeLabels(*input.Interests, "interests")
		if err != nil {
			return nil, err
		}
		input.Interests = &values
	}
	if input.PrimaryImageURL != nil {
		value := strings.TrimSpace(*input.PrimaryImageURL)
		if !validMediaURL(value) {
			return nil, &fieldError{"primary_image_url", "Primary image must be an HTTP URL or a server media path."}
		}
		input.PrimaryImageURL = &value
	}
	if input.Location != nil && !validCoordinates(input.Location.Latitude, input.Location.Longitude) {
		return nil, &fieldError{"location", "Latitude must be between -90 and 90 and longitude between -180 and 180."}
	}
	if input.Status != nil {
		value := strings.ToLower(strings.TrimSpace(*input.Status))
		if _, ok := allowedStatuses[value]; !ok {
			return nil, &fieldError{"status", "Status must be active, paused, or adopted."}
		}
		input.Status = &value
	}
	return parseBirthDate(input.BirthDate, now)
}

func parseBirthDate(value *string, now time.Time) (*time.Time, *fieldError) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	*value = trimmed
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return nil, &fieldError{"birth_date", "Birth date must use YYYY-MM-DD."}
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if parsed.After(today) {
		return nil, &fieldError{"birth_date", "Birth date cannot be in the future."}
	}
	return &parsed, nil
}

func normalizeLabels(values []string, field string) ([]string, *fieldError) {
	if len(values) > 20 {
		return nil, &fieldError{field, "Choose no more than 20 values."}
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" || len([]rune(value)) > 50 {
			return nil, &fieldError{field, "Each value must contain between 1 and 50 characters."}
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func validateName(value string) *fieldError {
	if value == "" || len([]rune(value)) > 80 {
		return &fieldError{"name", "Pet name must contain between 1 and 80 characters."}
	}
	return nil
}

func validatePetType(value string) *fieldError {
	if _, ok := allowedPetTypes[value]; !ok {
		return &fieldError{"pet_type", "Pet type must be dog, cat, bird, rabbit, exotic, or other."}
	}
	return nil
}

func hasPatch(input patchRequest) bool {
	return input.Name != nil || input.PetType != nil || input.Breed != nil ||
		input.BirthDate != nil || input.AgeLabel != nil || input.Gender != nil ||
		input.WeightKG != nil || input.ClearWeight || input.Bio != nil ||
		input.Personality != nil || input.Interests != nil ||
		input.PrimaryImageURL != nil || input.Location != nil || input.ClearLocation ||
		input.Status != nil
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

func authenticatedUser(c *fiber.Ctx) (uuid.UUID, bool) {
	userID, err := httpx.UserID(c)
	return userID, err == nil
}

func parsePetID(c *fiber.Ctx) (uuid.UUID, error) {
	return httpx.UUIDParam(c, "petId")
}

func authenticationProblem(c *fiber.Ctx) error {
	return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
}

func invalidPetIDProblem(c *fiber.Ctx) error {
	return httpx.Problem(c, fiber.StatusBadRequest, "invalid_pet_id", "Pet ID must be a valid UUID.")
}

func notFoundProblem(c *fiber.Ctx) error {
	return httpx.Problem(c, fiber.StatusNotFound, "pet_not_found", "The pet was not found.")
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
