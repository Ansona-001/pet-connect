package httpx

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"petconnect/server/internal/platform/authjwt"
)

const userIDKey = "authenticated_user_id"

type ErrorPayload struct {
	Error APIError `json:"error"`
}

type APIError struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
}

func OK(c *fiber.Ctx, data any) error {
	return c.JSON(fiber.Map{"data": data})
}

func Created(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"data": data})
}

func Problem(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(ErrorPayload{Error: APIError{
		Code: code, Message: message, RequestID: c.GetRespHeader(fiber.HeaderXRequestID),
	}})
}

func UserID(c *fiber.Ctx) (uuid.UUID, error) {
	value, ok := c.Locals(userIDKey).(uuid.UUID)
	if !ok || value == uuid.Nil {
		return uuid.Nil, errors.New("authenticated user is missing")
	}
	return value, nil
}

func Authenticate(tokens *authjwt.Manager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
		if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
			return Problem(c, fiber.StatusUnauthorized, "authentication_required", "A valid access token is required.")
		}
		claims, err := tokens.Parse(strings.TrimSpace(header[7:]))
		if err != nil {
			return Problem(c, fiber.StatusUnauthorized, "invalid_access_token", "The access token is invalid or expired.")
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return Problem(c, fiber.StatusUnauthorized, "invalid_access_token", "The access token subject is invalid.")
		}
		c.Locals(userIDKey, userID)
		return c.Next()
	}
}

func UUIDParam(c *fiber.Ctx, name string) (uuid.UUID, error) {
	return uuid.Parse(c.Params(name))
}
