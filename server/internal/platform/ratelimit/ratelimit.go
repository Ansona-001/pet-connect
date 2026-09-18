// Package ratelimit provides a small Redis-backed fixed-window request
// limiter for Fiber routes — brief defect #5 ("distributed rate
// limiting"), previously only stood in for by per-account cooldowns in
// the verification/password-reset handlers (see their doc comments).
package ratelimit

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"petconnect/server/internal/platform/httpx"
)

const keyPrefix = "petconnect:ratelimit:"

// Limit returns Fiber middleware allowing at most max requests per window
// from the same key (as computed by keyFunc), using a fixed-window
// counter — INCR the window's key, set its expiry on first use, and
// reject once the count exceeds max. A fixed window is simpler than a
// sliding one and, for abuse protection rather than precise fairness,
// the boundary imprecision it trades away (a burst spanning two windows
// could momentarily allow closer to 2x max) is an acceptable cost.
//
// name namespaces the counter per call site so the same client can be
// independently limited on /auth/login and /auth/register at once.
// Every rejection returns the identical generic "rate_limited" response
// regardless of *why* the caller has been making requests — a client
// probing valid vs. invalid credentials must not be able to distinguish
// "wrong password" from "rate limited" by response shape, the same
// enumeration-safety reasoning the brief applies to auth error messages
// generally (brief §13).
func Limit(redisClient *redis.Client, name string, max int, window time.Duration, keyFunc func(c *fiber.Ctx) string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := keyPrefix + name + ":" + keyFunc(c)
		count, err := redisClient.Incr(c.UserContext(), key).Result()
		if err != nil {
			// Fail open: a Redis outage must not take the auth surface down
			// entirely — the same tradeoff httpx.Authenticate makes for its
			// session-invalidation lookup.
			return c.Next()
		}
		if count == 1 {
			// Only the request that created the counter sets its expiry, so
			// a lost race here (two requests both seeing count==1, both
			// setting EXPIRE) is harmless — both set it to the same window.
			redisClient.Expire(c.UserContext(), key, window)
		}
		if count > int64(max) {
			c.Set(fiber.HeaderRetryAfter, strconv.Itoa(int(window.Seconds())))
			return httpx.Problem(c, fiber.StatusTooManyRequests, "rate_limited", "Too many requests. Please try again later.")
		}
		return c.Next()
	}
}

// KeyByIP rate-limits per client IP — the default choice for public,
// unauthenticated auth endpoints where there's no other stable identity
// to key on.
func KeyByIP(c *fiber.Ctx) string {
	return c.IP()
}
