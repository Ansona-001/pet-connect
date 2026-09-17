package httpx

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"petconnect/server/internal/platform/authjwt"
)

const (
	userIDKey        = "authenticated_user_id"
	refreshRecordKey = "authenticated_refresh_record_id"
)

// SessionInvalidationKey is the Redis key holding the cutoff timestamp
// (RFC3339Nano) below which a user's existing access tokens are no longer
// honored — written by logout-all, checked here by Authenticate. See ADR
// 0004 (docs/adr/0004-auth-sessions.md). Revoking a single session doesn't
// touch this key: an already-issued access token for that one session is
// accepted, per ADR 0004, to expire naturally within its short TTL — the
// same documented latency plain logout already has. Logout-all raises the
// bar because it's a broader, more security-sensitive action.
func SessionInvalidationKey(userID uuid.UUID) string {
	return "petconnect:auth:invalidated_after:" + userID.String()
}

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

// RefreshRecordID returns the refresh_tokens.id that was current when this
// access token was issued (the JWT's `sid` claim) — not a `sessions` table
// id. Handlers that need to know which durable session (sessions row) the
// current request belongs to look up that row's family_id by this id and
// compare against sessions.family_id; see auth.listSessions.
func RefreshRecordID(c *fiber.Ctx) (uuid.UUID, error) {
	value, ok := c.Locals(refreshRecordKey).(uuid.UUID)
	if !ok || value == uuid.Nil {
		return uuid.Nil, errors.New("authenticated session is missing")
	}
	return value, nil
}

func Authenticate(tokens *authjwt.Manager, redisClient *redis.Client) fiber.Handler {
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
		refreshRecordID, err := uuid.Parse(claims.SessionID)
		if err != nil {
			return Problem(c, fiber.StatusUnauthorized, "invalid_access_token", "The access token session is invalid.")
		}

		if claims.IssuedAt != nil {
			cutoff, err := redisClient.Get(c.UserContext(), SessionInvalidationKey(userID)).Result()
			// A missing key (redis.Nil) or any Redis error both fall
			// through to allow the request: this check exists to tighten
			// security on logout-all, not to take the whole API down
			// whenever Redis briefly hiccups.
			if err == nil {
				if parsedCutoff, parseErr := time.Parse(time.RFC3339Nano, cutoff); parseErr == nil {
					if claims.IssuedAt.Time.Before(parsedCutoff) {
						return Problem(c, fiber.StatusUnauthorized, "invalid_access_token", "The access token is invalid or expired.")
					}
				}
			}
		}

		c.Locals(userIDKey, userID)
		c.Locals(refreshRecordKey, refreshRecordID)
		return c.Next()
	}
}

func UUIDParam(c *fiber.Ctx, name string) (uuid.UUID, error) {
	return uuid.Parse(c.Params(name))
}
