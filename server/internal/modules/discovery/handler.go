package discovery

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"petconnect/server/internal/platform/httpx"
)

type Handler struct {
	db *pgxpool.Pool
}

type Candidate struct {
	ID            uuid.UUID `json:"id"`
	OwnerID       uuid.UUID `json:"owner_id"`
	OwnerName     string    `json:"owner_name"`
	Name          string    `json:"name"`
	PetType       string    `json:"pet_type"`
	Breed         string    `json:"breed"`
	AgeLabel      string    `json:"age_label"`
	Gender        string    `json:"gender"`
	Bio           string    `json:"bio"`
	Personality   []string  `json:"personality"`
	Interests     []string  `json:"interests"`
	ImageURL      string    `json:"primary_image_url"`
	IsVerified    bool      `json:"is_verified"`
	DistanceKM    float64   `json:"distance_km"`
	FollowerCount int64     `json:"follower_count"`
	FollowingByMe bool      `json:"following_by_me"`
}

type candidatePage struct {
	Items      []Candidate `json:"items"`
	NextCursor string      `json:"next_cursor,omitempty"`
}

type distanceCursor struct {
	Meters float64
	ID     uuid.UUID
}

func (h *Handler) nearbyPets(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
	}
	limit, err := discoveryLimit(c.Query("limit"))
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", err.Error())
	}
	radiusKM, err := boundedFloat(c.Query("radius_km"), 25, 0.1, 100)
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_radius", "radius_km must be between 0.1 and 100.")
	}

	var latitude any
	var longitude any
	latRaw := strings.TrimSpace(c.Query("lat"))
	lngRaw := strings.TrimSpace(c.Query("lng"))
	if (latRaw == "") != (lngRaw == "") {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_location", "lat and lng must be supplied together.")
	}
	if latRaw != "" {
		lat, parseErr := strconv.ParseFloat(latRaw, 64)
		if parseErr != nil || lat < -90 || lat > 90 {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_latitude", "lat must be between -90 and 90.")
		}
		lng, parseErr := strconv.ParseFloat(lngRaw, 64)
		if parseErr != nil || lng < -180 || lng > 180 {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_longitude", "lng must be between -180 and 180.")
		}
		latitude, longitude = lat, lng
	}

	var sourcePetID any
	if raw := strings.TrimSpace(c.Query("source_pet_id")); raw != "" {
		parsed, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_source_pet_id", "source_pet_id must be a valid UUID.")
		}
		var owned bool
		if err := h.db.QueryRow(c.UserContext(), `
			SELECT EXISTS (SELECT 1 FROM pets WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL AND status <> 'deleted')`, parsed, userID).Scan(&owned); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "discovery_query_failed", "Nearby pets could not be loaded.")
		}
		if !owned {
			return httpx.Problem(c, fiber.StatusForbidden, "pet_not_owned", "The selected source pet does not belong to the authenticated user.")
		}
		sourcePetID = parsed
	}

	if latitude == nil {
		var hasLocation bool
		if sourcePetID != nil {
			err = h.db.QueryRow(c.UserContext(), "SELECT location IS NOT NULL FROM pets WHERE id = $1", sourcePetID).Scan(&hasLocation)
		} else {
			err = h.db.QueryRow(c.UserContext(), "SELECT location IS NOT NULL FROM users WHERE id = $1 AND deleted_at IS NULL", userID).Scan(&hasLocation)
		}
		if err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "discovery_query_failed", "Nearby pets could not be loaded.")
		}
		if !hasLocation {
			return httpx.Problem(c, fiber.StatusUnprocessableEntity, "location_required", "A location is required to discover nearby pets.")
		}
	}

	var cursorMeters any
	var cursorID any
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		cursor, decodeErr := decodeDistanceCursor(raw)
		if decodeErr != nil {
			return httpx.Problem(c, fiber.StatusBadRequest, "invalid_cursor", "The pagination cursor is invalid.")
		}
		cursorMeters, cursorID = cursor.Meters, cursor.ID
	}

	petType := strings.ToLower(strings.TrimSpace(c.Query("type")))
	breed := strings.TrimSpace(c.Query("breed"))
	gender := strings.ToLower(strings.TrimSpace(c.Query("gender")))
	query := strings.TrimSpace(c.Query("q"))
	if gender != "" && gender != "male" && gender != "female" && gender != "unknown" {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_gender", "gender must be male, female, or unknown.")
	}

	rows, err := h.db.Query(c.UserContext(), `
		WITH origin AS (
		  SELECT COALESCE(
		    CASE WHEN $2::double precision IS NOT NULL
		      THEN ST_SetSRID(ST_MakePoint($3::double precision, $2::double precision), 4326)::geography
		    END,
		    (SELECT source.location FROM pets source WHERE source.id = $4::uuid),
		    u.location
		  ) AS location
		  FROM users u WHERE u.id = $1
		), nearby AS (
		  SELECT p.id, p.owner_id, owner.name AS owner_name, p.name, p.pet_type,
		         p.breed, p.age_label, p.gender, p.bio, p.personality, p.interests,
		         p.primary_image_url, p.is_verified,
		         ST_Distance(p.location, origin.location) AS distance_meters,
		         (SELECT count(*) FROM follows f WHERE f.pet_id = p.id) AS follower_count,
		         EXISTS (SELECT 1 FROM follows f WHERE f.pet_id = p.id AND f.user_id = $1) AS following_by_me
		  FROM pets p
		  JOIN users owner ON owner.id = p.owner_id
		  CROSS JOIN origin
		  WHERE origin.location IS NOT NULL AND p.location IS NOT NULL
		    AND p.owner_id <> $1 AND p.deleted_at IS NULL AND p.status = 'active'
		    AND owner.deleted_at IS NULL
		    -- Enforced both directions — see internal/modules/social's feed
		    -- query for the same pattern and reasoning.
		    AND NOT EXISTS (
		      SELECT 1 FROM blocks bl
		      WHERE (bl.blocker_user_id = $1 AND bl.blocked_user_id = p.owner_id)
		         OR (bl.blocker_user_id = p.owner_id AND bl.blocked_user_id = $1)
		    )
		    AND ST_DWithin(p.location, origin.location, $5::double precision)
		    AND ($6 = '' OR lower(p.pet_type) = $6)
		    AND ($7 = '' OR p.breed ILIKE '%' || $7 || '%')
		    AND ($8 = '' OR p.gender = $8)
		    AND ($9 = '' OR p.name ILIKE '%' || $9 || '%' OR p.breed ILIKE '%' || $9 || '%')
		    AND ($4::uuid IS NULL OR NOT EXISTS (
		      SELECT 1 FROM pet_swipes sw WHERE sw.source_pet_id = $4::uuid AND sw.target_pet_id = p.id
		    ))
		)
		SELECT id, owner_id, owner_name, name, pet_type, breed, age_label, gender,
		       bio, personality, interests, primary_image_url, is_verified,
		       distance_meters, follower_count, following_by_me
		FROM nearby
		WHERE ($10::double precision IS NULL OR (distance_meters, id) > ($10::double precision, $11::uuid))
		ORDER BY distance_meters ASC, id ASC
		LIMIT $12`, userID, latitude, longitude, sourcePetID, radiusKM*1000, petType, breed, gender, query, cursorMeters, cursorID, limit+1)
	if err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "discovery_query_failed", "Nearby pets could not be loaded.")
	}
	defer rows.Close()

	items := make([]Candidate, 0, limit)
	distances := make([]float64, 0, limit+1)
	for rows.Next() {
		var item Candidate
		var distanceMeters float64
		if err := rows.Scan(
			&item.ID, &item.OwnerID, &item.OwnerName, &item.Name, &item.PetType,
			&item.Breed, &item.AgeLabel, &item.Gender, &item.Bio, &item.Personality,
			&item.Interests, &item.ImageURL, &item.IsVerified, &distanceMeters,
			&item.FollowerCount, &item.FollowingByMe,
		); err != nil {
			return httpx.Problem(c, fiber.StatusInternalServerError, "discovery_query_failed", "Nearby pets could not be loaded.")
		}
		item.DistanceKM = distanceMeters / 1000
		items = append(items, item)
		distances = append(distances, distanceMeters)
	}
	if err := rows.Err(); err != nil {
		return httpx.Problem(c, fiber.StatusInternalServerError, "discovery_query_failed", "Nearby pets could not be loaded.")
	}

	nextCursor := ""
	if len(items) > limit {
		items = items[:limit]
		distances = distances[:limit]
		last := items[len(items)-1]
		nextCursor = encodeDistanceCursor(distances[len(distances)-1], last.ID)
	}
	return httpx.OK(c, candidatePage{Items: items, NextCursor: nextCursor})
}

func discoveryLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 20, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 50 {
		return 0, fmt.Errorf("limit must be between 1 and 50")
	}
	return value, nil
}

func boundedFloat(raw string, fallback, minimum, maximum float64) (float64, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < minimum || value > maximum {
		return 0, fmt.Errorf("value must be between %g and %g", minimum, maximum)
	}
	return value, nil
}

func encodeDistanceCursor(meters float64, id uuid.UUID) string {
	value := strconv.FormatFloat(meters, 'f', 6, 64) + "|" + id.String()
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeDistanceCursor(raw string) (distanceCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return distanceCursor{}, err
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return distanceCursor{}, fmt.Errorf("invalid cursor")
	}
	meters, err := strconv.ParseFloat(parts[0], 64)
	if err != nil || meters < 0 {
		return distanceCursor{}, fmt.Errorf("invalid distance")
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return distanceCursor{}, err
	}
	return distanceCursor{Meters: meters, ID: id}, nil
}
