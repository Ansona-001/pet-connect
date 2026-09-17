package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"petconnect/server/internal/platform/authjwt"
	"petconnect/server/internal/platform/httpx"
	"petconnect/server/internal/platform/mailer"
)

const (
	minimumPasswordBytes = 8
	maximumPasswordBytes = 72
)

var (
	errInvalidCredentials = errors.New("invalid credentials")
	errInvalidRefresh     = errors.New("invalid refresh token")
	errRefreshReused      = errors.New("refresh token reused")
)

// Handler owns email/password authentication and refresh-token sessions.
type Handler struct {
	db         *pgxpool.Pool
	tokens     *authjwt.Manager
	refreshTTL time.Duration
	clock      func() time.Time
	mailer     mailer.Sender
	redis      *redis.Client
}

// New constructs the authentication module.
func New(db *pgxpool.Pool, tokens *authjwt.Manager, refreshTTL time.Duration, sender mailer.Sender, redisClient *redis.Client) *Handler {
	return &Handler{db: db, tokens: tokens, refreshTTL: refreshTTL, clock: time.Now, mailer: sender, redis: redisClient}
}

// RegisterRoutes mounts public authentication routes on a router rooted at /v1.
func RegisterRoutes(router fiber.Router, db *pgxpool.Pool, tokens *authjwt.Manager, refreshTTL time.Duration, sender mailer.Sender, redisClient *redis.Client) *Handler {
	handler := New(db, tokens, refreshTTL, sender, redisClient)
	handler.RegisterRoutes(router)
	return handler
}

// RegisterRoutes mounts this handler's public routes.
func (h *Handler) RegisterRoutes(router fiber.Router) {
	auth := router.Group("/auth")
	auth.Post("/register", h.register)
	auth.Post("/login", h.login)
	auth.Post("/refresh", h.refresh)
	auth.Post("/logout", h.logout)
	auth.Post("/verify-email", h.verifyEmail)
	auth.Post("/resend-verification", h.resendVerification)
	auth.Post("/forgot-password", h.forgotPassword)
	auth.Post("/reset-password", h.resetPassword)
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type userResponse struct {
	ID                  uuid.UUID  `json:"id"`
	Email               string     `json:"email"`
	Name                string     `json:"name"`
	Bio                 string     `json:"bio"`
	City                string     `json:"city"`
	ProfilePhotoURL     string     `json:"profile_photo_url"`
	OnboardingCompleted bool       `json:"onboarding_completed"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	OnboardedAt         *time.Time `json:"onboarding_completed_at,omitempty"`
}

type tokenResponse struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	TokenType    string        `json:"token_type"`
	ExpiresAt    time.Time     `json:"expires_at"`
	User         *userResponse `json:"user,omitempty"`
}

type refreshRecord struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	FamilyID  uuid.UUID
	ExpiresAt time.Time
	UsedAt    *time.Time
	RevokedAt *time.Time
}

func (h *Handler) register(c *fiber.Ctx) error {
	var input registerRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}

	input.Email = normalizeEmail(input.Email)
	input.Name = strings.TrimSpace(input.Name)
	if field, message := validateRegistration(input); field != "" {
		return validationProblem(c, field, message)
	}

	tx, err := h.db.BeginTx(c.UserContext(), pgx.TxOptions{})
	if err != nil {
		return internalProblem(c)
	}
	defer func() { _ = tx.Rollback(c.UserContext()) }()

	var user userResponse
	err = tx.QueryRow(c.UserContext(), `
		INSERT INTO users (email, password_hash, name)
		VALUES ($1, crypt($2, gen_salt('bf', 12)), $3)
		RETURNING id, email::text, name, bio, city, profile_photo_url,
		          onboarding_completed_at IS NOT NULL, created_at, updated_at,
		          onboarding_completed_at`, input.Email, input.Password, input.Name).Scan(
		&user.ID, &user.Email, &user.Name, &user.Bio, &user.City,
		&user.ProfilePhotoURL, &user.OnboardingCompleted, &user.CreatedAt,
		&user.UpdatedAt, &user.OnboardedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return httpx.Problem(c, fiber.StatusConflict, "email_in_use", "An account already exists for that email address.")
		}
		return internalProblem(c)
	}

	result, err := h.createSession(c.UserContext(), tx, user.ID, &user, c.Get(fiber.HeaderUserAgent))
	if err != nil {
		return internalProblem(c)
	}

	// Issued in the same transaction as the account row, so the two are
	// atomic — a client never sees a created account with no way to
	// verify it. Sending is best-effort and happens after commit: no
	// transactional email provider is configured yet (brief §18), so this
	// currently just logs the token (mailer.LogSender) without failing
	// registration.
	verificationToken, err := h.issueEmailVerification(c.UserContext(), tx, user.ID, h.clock().UTC())
	if err != nil {
		return internalProblem(c)
	}

	if err := tx.Commit(c.UserContext()); err != nil {
		return internalProblem(c)
	}
	_ = h.mailer.SendVerificationEmail(user.Email, verificationToken)

	c.Set(fiber.HeaderCacheControl, "no-store")
	return httpx.Created(c, result)
}

func (h *Handler) login(c *fiber.Ctx) error {
	var input loginRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	input.Email = normalizeEmail(input.Email)
	if input.Email == "" || input.Password == "" {
		return httpx.Problem(c, fiber.StatusUnauthorized, "invalid_credentials", "The email or password is incorrect.")
	}

	tx, err := h.db.BeginTx(c.UserContext(), pgx.TxOptions{})
	if err != nil {
		return internalProblem(c)
	}
	defer func() { _ = tx.Rollback(c.UserContext()) }()

	user, err := findUserByCredentials(c.UserContext(), tx, input.Email, input.Password)
	if err != nil {
		if errors.Is(err, errInvalidCredentials) {
			return httpx.Problem(c, fiber.StatusUnauthorized, "invalid_credentials", "The email or password is incorrect.")
		}
		return internalProblem(c)
	}

	result, err := h.createSession(c.UserContext(), tx, user.ID, &user, c.Get(fiber.HeaderUserAgent))
	if err != nil {
		return internalProblem(c)
	}
	if err := tx.Commit(c.UserContext()); err != nil {
		return internalProblem(c)
	}

	c.Set(fiber.HeaderCacheControl, "no-store")
	return httpx.OK(c, result)
}

func (h *Handler) refresh(c *fiber.Ctx) error {
	var input refreshRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	input.RefreshToken = strings.TrimSpace(input.RefreshToken)
	if input.RefreshToken == "" {
		return validationProblem(c, "refresh_token", "Refresh token is required.")
	}

	result, err := h.rotate(c.UserContext(), input.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, errRefreshReused):
			return httpx.Problem(c, fiber.StatusUnauthorized, "refresh_token_reused", "This refresh-token session has been revoked.")
		case errors.Is(err, errInvalidRefresh):
			return httpx.Problem(c, fiber.StatusUnauthorized, "invalid_refresh_token", "The refresh token is invalid or expired.")
		default:
			return internalProblem(c)
		}
	}

	c.Set(fiber.HeaderCacheControl, "no-store")
	return httpx.OK(c, result)
}

func (h *Handler) logout(c *fiber.Ctx) error {
	var input refreshRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	input.RefreshToken = strings.TrimSpace(input.RefreshToken)
	if input.RefreshToken == "" {
		return validationProblem(c, "refresh_token", "Refresh token is required.")
	}

	familyID := uuid.Nil
	if err := h.db.QueryRow(c.UserContext(), `
		SELECT family_id FROM refresh_tokens WHERE token_hash = $1`,
		authjwt.HashOpaqueToken(input.RefreshToken)).Scan(&familyID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return internalProblem(c)
	}

	if _, err := h.db.Exec(c.UserContext(), `
		UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, now())
		WHERE family_id = $1`, familyID); err != nil {
		return internalProblem(c)
	}
	// Keeps the sessions list (GET /v1/sessions) honest — without this, a
	// session ended via plain logout would still show up there as active.
	if _, err := h.db.Exec(c.UserContext(), `
		UPDATE sessions SET revoked_at = COALESCE(revoked_at, now())
		WHERE family_id = $1`, familyID); err != nil {
		return internalProblem(c)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) createSession(ctx context.Context, tx pgx.Tx, userID uuid.UUID, user *userResponse, userAgent string) (tokenResponse, error) {
	plain, hash, err := authjwt.NewOpaqueToken()
	if err != nil {
		return tokenResponse{}, err
	}

	now := h.clock().UTC()
	recordID := uuid.New()
	familyID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)`, recordID, userID, familyID, hash, now.Add(h.refreshTTL))
	if err != nil {
		return tokenResponse{}, err
	}

	// A device row per sign-in, not deduplicated against prior ones: there's
	// no client-supplied device identifier to correlate "same device, new
	// login" against, so each login/register is its own row in the session
	// list. Good enough to let a user recognize and revoke a stray sign-in;
	// real device fingerprinting would be its own feature.
	deviceName, platform := deviceLabelFromUserAgent(userAgent)
	var deviceID uuid.UUID
	if err := tx.QueryRow(ctx, `
		INSERT INTO devices (user_id, name, platform)
		VALUES ($1, $2, $3)
		RETURNING id`, userID, deviceName, platform).Scan(&deviceID); err != nil {
		return tokenResponse{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO sessions (user_id, device_id, family_id)
		VALUES ($1, $2, $3)`, userID, deviceID, familyID); err != nil {
		return tokenResponse{}, err
	}

	access, expiresAt, err := h.tokens.Issue(userID, recordID)
	if err != nil {
		return tokenResponse{}, err
	}
	return tokenResponse{
		AccessToken: access, RefreshToken: plain, TokenType: "Bearer",
		ExpiresAt: expiresAt, User: user,
	}, nil
}

// deviceLabelFromUserAgent makes a best-effort, dependency-free guess at a
// human-readable device name and a coarse platform from the User-Agent
// header. It's deliberately simple substring matching, not a full UA
// parser — good enough for a session list ("Windows", "iOS"), not intended
// for anything security-sensitive.
func deviceLabelFromUserAgent(userAgent string) (name, platform string) {
	userAgent = strings.TrimSpace(userAgent)
	if userAgent == "" {
		return "Unknown device", ""
	}
	name = userAgent
	if len(name) > 120 {
		name = name[:120]
	}
	lower := strings.ToLower(userAgent)
	switch {
	case strings.Contains(lower, "android"):
		platform = "android"
	case strings.Contains(lower, "iphone"), strings.Contains(lower, "ipad"):
		platform = "ios"
	case strings.Contains(lower, "windows"):
		platform = "windows"
	case strings.Contains(lower, "mac os"), strings.Contains(lower, "macintosh"):
		platform = "macos"
	case strings.Contains(lower, "linux"):
		platform = "linux"
	default:
		platform = "web"
	}
	return name, platform
}

func (h *Handler) rotate(ctx context.Context, plain string) (tokenResponse, error) {
	tx, err := h.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return tokenResponse{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var current refreshRecord
	err = tx.QueryRow(ctx, `
		SELECT rt.id, rt.user_id, rt.family_id, rt.expires_at, rt.used_at, rt.revoked_at
		FROM refresh_tokens rt
		JOIN users u ON u.id = rt.user_id
		WHERE rt.token_hash = $1 AND u.deleted_at IS NULL
		FOR UPDATE OF rt`, authjwt.HashOpaqueToken(plain)).Scan(
		&current.ID, &current.UserID, &current.FamilyID, &current.ExpiresAt,
		&current.UsedAt, &current.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return tokenResponse{}, errInvalidRefresh
	}
	if err != nil {
		return tokenResponse{}, err
	}

	if current.UsedAt != nil || current.RevokedAt != nil {
		if _, err := tx.Exec(ctx, `
			UPDATE refresh_tokens SET revoked_at = COALESCE(revoked_at, now())
			WHERE family_id = $1`, current.FamilyID); err != nil {
			return tokenResponse{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE sessions SET revoked_at = COALESCE(revoked_at, now())
			WHERE family_id = $1`, current.FamilyID); err != nil {
			return tokenResponse{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return tokenResponse{}, err
		}
		return tokenResponse{}, errRefreshReused
	}

	now := h.clock().UTC()
	if !current.ExpiresAt.After(now) {
		if _, err := tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`, current.ID); err != nil {
			return tokenResponse{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return tokenResponse{}, err
		}
		return tokenResponse{}, errInvalidRefresh
	}

	nextPlain, nextHash, err := authjwt.NewOpaqueToken()
	if err != nil {
		return tokenResponse{}, err
	}
	nextID := uuid.New()
	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, family_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5)`, nextID, current.UserID, current.FamilyID,
		nextHash, now.Add(h.refreshTTL))
	if err != nil {
		return tokenResponse{}, err
	}
	command, err := tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET used_at = $2, replaced_by = $3
		WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL`, current.ID, now, nextID)
	if err != nil || command.RowsAffected() != 1 {
		if err == nil {
			err = errRefreshReused
		}
		return tokenResponse{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE sessions SET last_used_at = $2 WHERE family_id = $1`, current.FamilyID, now); err != nil {
		return tokenResponse{}, err
	}

	access, expiresAt, err := h.tokens.Issue(current.UserID, nextID)
	if err != nil {
		return tokenResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return tokenResponse{}, err
	}

	return tokenResponse{
		AccessToken: access, RefreshToken: nextPlain, TokenType: "Bearer", ExpiresAt: expiresAt,
	}, nil
}

func findUserByCredentials(ctx context.Context, tx pgx.Tx, email, password string) (userResponse, error) {
	var user userResponse
	err := tx.QueryRow(ctx, `
		SELECT id, email::text, name, bio, city, profile_photo_url,
		       onboarding_completed_at IS NOT NULL, created_at, updated_at,
		       onboarding_completed_at
		FROM users
		WHERE email = $1
		  AND password_hash = crypt($2, password_hash)
		  AND deleted_at IS NULL`, email, password).Scan(
		&user.ID, &user.Email, &user.Name, &user.Bio, &user.City,
		&user.ProfilePhotoURL, &user.OnboardingCompleted, &user.CreatedAt,
		&user.UpdatedAt, &user.OnboardedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return userResponse{}, errInvalidCredentials
	}
	return user, err
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validateRegistration(input registerRequest) (string, string) {
	address, err := mail.ParseAddress(input.Email)
	if err != nil || !strings.EqualFold(address.Address, input.Email) || len(input.Email) > 320 {
		return "email", "Enter a valid email address."
	}
	if input.Name == "" || len([]rune(input.Name)) > 100 {
		return "name", "Name must contain between 1 and 100 characters."
	}
	passwordLength := len([]byte(input.Password))
	if passwordLength < minimumPasswordBytes || passwordLength > maximumPasswordBytes {
		return "password", "Password must contain between 8 and 72 bytes."
	}
	return "", ""
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
