package matching

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"petconnect/server/internal/platform/httpx"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register mounts matching routes on an already authenticated /v1 router.
func (h *Handler) Register(router fiber.Router) {
	router.Get("/pets/:petId/candidates", h.candidates)
	router.Post("/pets/:petId/swipes", h.swipe)
	router.Get("/matches", h.listMatches)
}

func RegisterRoutes(router fiber.Router, service *Service) {
	NewHandler(service).Register(router)
}

func (h *Handler) candidates(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "Authentication is required.")
	}
	petID, err := httpx.UUIDParam(c, "petId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_pet_id", "The pet id is invalid.")
	}
	maxDistance, err := parseFloat(c.Query("max_distance_km"), 0)
	if err != nil || maxDistance < 0 || maxDistance > 500 {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_distance", "max_distance_km must be between 0 and 500.")
	}
	limit, err := parseInt(c.Query("limit"), 20)
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", "The result limit is invalid.")
	}
	candidates, err := h.service.Candidates(c.UserContext(), userID, petID, CandidateFilter{
		PetType: c.Query("pet_type"), Breed: c.Query("breed"), Gender: c.Query("gender"),
		MaxDistanceKM: maxDistance, Limit: limit,
	})
	if err != nil {
		return matchingProblem(c, err)
	}
	return httpx.OK(c, fiber.Map{"items": candidates})
}

type swipePayload struct {
	TargetPetID     string `json:"target_pet_id"`
	Decision        string `json:"decision"`
	ClientRequestID string `json:"client_request_id"`
}

func (h *Handler) swipe(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "Authentication is required.")
	}
	sourceID, err := httpx.UUIDParam(c, "petId")
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_pet_id", "The source pet id is invalid.")
	}
	var payload swipePayload
	if err := c.BodyParser(&payload); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is invalid.")
	}
	targetID, targetErr := uuid.Parse(strings.TrimSpace(payload.TargetPetID))
	requestID, requestErr := uuid.Parse(strings.TrimSpace(payload.ClientRequestID))
	if targetErr != nil || requestErr != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_swipe", "target_pet_id and client_request_id must be UUIDs.")
	}
	result, err := h.service.Swipe(c.UserContext(), userID, SwipeRequest{
		SourcePetID: sourceID, TargetPetID: targetID,
		Decision:        SwipeDecision(strings.ToLower(strings.TrimSpace(payload.Decision))),
		ClientRequestID: requestID,
	})
	if err != nil {
		return matchingProblem(c, err)
	}
	return httpx.OK(c, result)
}

func (h *Handler) listMatches(c *fiber.Ctx) error {
	userID, err := httpx.UserID(c)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "authentication_required", "Authentication is required.")
	}
	limit, err := parseInt(c.Query("limit"), 50)
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_limit", "The result limit is invalid.")
	}
	matches, err := h.service.ListMatches(c.UserContext(), userID, limit)
	if err != nil {
		return matchingProblem(c, err)
	}
	return httpx.OK(c, fiber.Map{"items": matches})
}

func matchingProblem(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, ErrPetNotFound):
		return httpx.Problem(c, fiber.StatusNotFound, "pet_not_found", "The requested pet was not found.")
	case errors.Is(err, ErrForbidden):
		return httpx.Problem(c, fiber.StatusForbidden, "pet_forbidden", "You cannot act for this pet.")
	case errors.Is(err, ErrIdempotencyConflict):
		return httpx.Problem(c, fiber.StatusConflict, "idempotency_conflict", "The client request id was already used for different swipe input.")
	case errors.Is(err, ErrAlreadySwiped):
		return httpx.Problem(c, fiber.StatusConflict, "already_swiped", "This pet has already been swiped.")
	case errors.Is(err, ErrInvalidSwipe):
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_swipe", "The swipe request is invalid.")
	default:
		return httpx.Problem(c, fiber.StatusInternalServerError, "matching_failed", "The matching request could not be completed.")
	}
}

func parseInt(value string, fallback int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

func parseFloat(value string, fallback float64) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	return strconv.ParseFloat(value, 64)
}
