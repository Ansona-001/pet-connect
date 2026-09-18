package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"petconnect/server/internal/platform/authjwt"
	"petconnect/server/internal/platform/httpx"
	"petconnect/server/internal/platform/oidc"
)

const (
	// oauthStateTTL bounds how long a user has to complete the Google
	// consent screen once redirected there.
	oauthStateTTL = 10 * time.Minute
	// oauthExchangeTicketTTL is short because the client is expected to
	// redeem the ticket within moments of the redirect landing — see
	// oauthExchange.
	oauthExchangeTicketTTL = 60 * time.Second

	oauthStateKeyPrefix    = "petconnect:auth:oauth:state:"
	oauthExchangeKeyPrefix = "petconnect:auth:oauth:exchange:"
)

var errOAuthEmailInUseUnverified = errors.New("oauth email in use by an unverified identity")

// providersResponse tells the client which sign-in buttons to show,
// keeping that decision server-side (see GoogleOAuthConfig) instead of
// duplicating "is this configured" logic into build-time client config.
type providersResponse struct {
	GoogleEnabled bool `json:"google_enabled"`
	AppleEnabled  bool `json:"apple_enabled"`
}

func (h *Handler) listProviders(c *fiber.Ctx) error {
	return httpx.OK(c, providersResponse{
		GoogleEnabled: h.google != nil,
		// Apple Sign-In lands in a later slice reusing this same oidc
		// package (ADR 0005) — always false until then.
		AppleEnabled: false,
	})
}

type authorizationURLResponse struct {
	AuthorizationURL string `json:"authorization_url"`
}

// googleStart begins the Authorization Code + PKCE flow. It returns the
// URL as JSON rather than issuing an HTTP redirect because the caller is
// a Flutter app deciding for itself how to open it (system browser or
// webview), not a browser following redirects automatically.
func (h *Handler) googleStart(c *fiber.Ctx) error {
	if h.google == nil {
		return httpx.Problem(c, fiber.StatusServiceUnavailable, "google_oauth_not_configured", "Google Sign-In is not configured on this server.")
	}

	state, _, err := authjwt.NewOpaqueToken()
	if err != nil {
		return internalProblem(c)
	}
	verifier, _, err := authjwt.NewOpaqueToken()
	if err != nil {
		return internalProblem(c)
	}
	challenge := pkceChallenge(verifier)

	if err := h.redis.Set(c.UserContext(), oauthStateKeyPrefix+state, verifier, oauthStateTTL).Err(); err != nil {
		return internalProblem(c)
	}

	query := url.Values{
		"client_id":             {h.google.clientID},
		"redirect_uri":          {h.google.redirectURL},
		"response_type":         {"code"},
		"scope":                 {"openid email profile"},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"prompt":                {"select_account"},
	}

	c.Set(fiber.HeaderCacheControl, "no-store")
	return httpx.OK(c, authorizationURLResponse{
		AuthorizationURL: googleAuthorizationEndpoint + "?" + query.Encode(),
	})
}

// googleCallback is where Google redirects the browser/webview back to
// after the user consents (or declines). On success it hands control
// back to the app via a single-use exchange ticket rather than putting
// access/refresh tokens directly in a redirect URL, which a device's
// browser history or an OS could otherwise retain.
//
// The petconnect:// deep link below is the intended eventual target, but
// registering that URL scheme in the Flutter app (and launching the
// browser/webview in the first place) is explicitly out of scope for
// this slice per the plan's own task list ("Flutter button hidden until
// client ID is configured") — the endpoint is fully functional and
// independently testable today, the last hop into the app is not.
func (h *Handler) googleCallback(c *fiber.Ctx) error {
	if h.google == nil {
		return httpx.Problem(c, fiber.StatusServiceUnavailable, "google_oauth_not_configured", "Google Sign-In is not configured on this server.")
	}

	if oauthErr := c.Query("error"); oauthErr != "" {
		return httpx.Problem(c, fiber.StatusBadRequest, "google_oauth_denied", "Google sign-in was cancelled or denied.")
	}
	code := strings.TrimSpace(c.Query("code"))
	state := strings.TrimSpace(c.Query("state"))
	if code == "" || state == "" {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_oauth_callback", "The OAuth callback is missing required parameters.")
	}

	verifier, err := h.redis.GetDel(c.UserContext(), oauthStateKeyPrefix+state).Result()
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_oauth_state", "This sign-in attempt has expired or was already used. Please try again.")
	}

	idToken, err := exchangeGoogleCode(c.UserContext(), h.google, code, verifier)
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadGateway, "google_token_exchange_failed", "Could not complete Google sign-in.")
	}

	claims, err := h.google.verifier.Verify(c.UserContext(), idToken)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "invalid_google_id_token", "Could not verify the Google identity token.")
	}

	result, err := h.linkOrCreateOAuthAccount(c.UserContext(), "google", claims, c.Get(fiber.HeaderUserAgent))
	if err != nil {
		if errors.Is(err, errOAuthEmailInUseUnverified) {
			return httpx.Problem(c, fiber.StatusConflict, "email_in_use", "An account already exists for that email address. Sign in with your password instead.")
		}
		return internalProblem(c)
	}

	ticket, _, err := authjwt.NewOpaqueToken()
	if err != nil {
		return internalProblem(c)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return internalProblem(c)
	}
	if err := h.redis.Set(c.UserContext(), oauthExchangeKeyPrefix+ticket, payload, oauthExchangeTicketTTL).Err(); err != nil {
		return internalProblem(c)
	}

	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Redirect("petconnect://auth/callback?ticket=" + url.QueryEscape(ticket))
}

type oauthExchangeRequest struct {
	Ticket string `json:"ticket"`
}

// oauthExchange redeems a single-use ticket minted by googleCallback for
// the actual access/refresh tokens. Single-use via GetDel: a replayed
// ticket (e.g. a deep link reopened from history) always misses the
// second time.
func (h *Handler) oauthExchange(c *fiber.Ctx) error {
	var input oauthExchangeRequest
	if err := c.BodyParser(&input); err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_json", "The request body is not valid JSON.")
	}
	input.Ticket = strings.TrimSpace(input.Ticket)
	if input.Ticket == "" {
		return validationProblem(c, "ticket", "Ticket is required.")
	}

	payload, err := h.redis.GetDel(c.UserContext(), oauthExchangeKeyPrefix+input.Ticket).Result()
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "invalid_ticket", "This sign-in ticket has expired or was already used.")
	}

	var result tokenResponse
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return internalProblem(c)
	}

	c.Set(fiber.HeaderCacheControl, "no-store")
	return httpx.OK(c, result)
}

// linkOrCreateOAuthAccount applies the safe account-linking rules (brief
// §13) shared by every OIDC provider — written against oidc.Claims, not
// a Google-specific type, so Apple Sign-In (Day 17) can call this
// unchanged with provider="apple":
//
//  1. An oauth_identities row already links this exact provider account
//     to a PetConnect user: log in as that user. No email comparison
//     needed or wanted here — the link was already established safely.
//  2. No existing link, but the provider vouches for a verified email
//     that matches an existing PetConnect account: auto-link. Safe
//     specifically because EmailVerified is required — an attacker
//     controlling an OIDC account with an unverified copy of someone
//     else's email must not be able to attach themselves to that
//     person's account.
//  3. No existing link and no verified-email match: create a fresh
//     account. An *unverified* email is fine to use for a brand-new
//     account (there is no existing account to take over), but if that
//     email is already taken by an existing account we must not silently
//     create a second row that collides with it or silently log the
//     caller into someone else's account — errOAuthEmailInUseUnverified
//     covers that one edge case explicitly.
func (h *Handler) linkOrCreateOAuthAccount(ctx context.Context, provider string, claims oidc.Claims, userAgent string) (tokenResponse, error) {
	tx, err := h.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return tokenResponse{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	user, err := findUserByOAuthIdentity(ctx, tx, provider, claims.Subject)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return tokenResponse{}, err
	}
	linked := err == nil

	if !linked && claims.Email != "" {
		email := normalizeEmail(claims.Email)
		existing, findErr := findUserByEmail(ctx, tx, email)
		switch {
		case findErr == nil && claims.EmailVerified:
			user = existing
			if err := linkOAuthIdentity(ctx, tx, provider, claims, user.ID); err != nil {
				return tokenResponse{}, err
			}
			linked = true
		case findErr == nil:
			return tokenResponse{}, errOAuthEmailInUseUnverified
		case !errors.Is(findErr, pgx.ErrNoRows):
			return tokenResponse{}, findErr
		}
	}

	if !linked {
		created, err := createUserFromOAuth(ctx, tx, claims)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return tokenResponse{}, errOAuthEmailInUseUnverified
			}
			return tokenResponse{}, err
		}
		user = created
		if err := linkOAuthIdentity(ctx, tx, provider, claims, user.ID); err != nil {
			return tokenResponse{}, err
		}
	}

	result, err := h.createSession(ctx, tx, user.ID, &user, userAgent)
	if err != nil {
		return tokenResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return tokenResponse{}, err
	}
	return result, nil
}

func findUserByOAuthIdentity(ctx context.Context, tx pgx.Tx, provider, providerUserID string) (userResponse, error) {
	var user userResponse
	err := tx.QueryRow(ctx, `
		SELECT u.id, u.email::text, u.name, u.bio, u.city, u.profile_photo_url,
		       u.onboarding_completed_at IS NOT NULL, u.created_at, u.updated_at,
		       u.onboarding_completed_at
		FROM oauth_identities oi
		JOIN users u ON u.id = oi.user_id
		WHERE oi.provider = $1 AND oi.provider_user_id = $2 AND u.deleted_at IS NULL`,
		provider, providerUserID).Scan(
		&user.ID, &user.Email, &user.Name, &user.Bio, &user.City,
		&user.ProfilePhotoURL, &user.OnboardingCompleted, &user.CreatedAt,
		&user.UpdatedAt, &user.OnboardedAt,
	)
	return user, err
}

func findUserByEmail(ctx context.Context, tx pgx.Tx, email string) (userResponse, error) {
	var user userResponse
	err := tx.QueryRow(ctx, `
		SELECT id, email::text, name, bio, city, profile_photo_url,
		       onboarding_completed_at IS NOT NULL, created_at, updated_at,
		       onboarding_completed_at
		FROM users WHERE email = $1 AND deleted_at IS NULL`, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.Bio, &user.City,
		&user.ProfilePhotoURL, &user.OnboardingCompleted, &user.CreatedAt,
		&user.UpdatedAt, &user.OnboardedAt,
	)
	return user, err
}

// createUserFromOAuth inserts a fresh account with no usable password:
// password_hash is random bytes rather than a bcrypt hash, which makes
// crypt(candidate, password_hash) in the login query fall back to a
// short DES-style hash that can never equal the 64-character hex string
// stored here — the same reasoning deletion.go already relies on for
// anonymized accounts. The user can set a real password later via
// forgot-password if they want password login in addition to Google.
func createUserFromOAuth(ctx context.Context, tx pgx.Tx, claims oidc.Claims) (userResponse, error) {
	name := strings.TrimSpace(claims.Name)
	if name == "" {
		if at := strings.IndexByte(claims.Email, '@'); at > 0 {
			name = claims.Email[:at]
		} else {
			name = "PetConnect user"
		}
	}
	if len([]rune(name)) > 100 {
		name = string([]rune(name)[:100])
	}

	var user userResponse
	err := tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, name)
		VALUES ($1, encode(gen_random_bytes(32), 'hex'), $2)
		RETURNING id, email::text, name, bio, city, profile_photo_url,
		          onboarding_completed_at IS NOT NULL, created_at, updated_at,
		          onboarding_completed_at`, normalizeEmail(claims.Email), name).Scan(
		&user.ID, &user.Email, &user.Name, &user.Bio, &user.City,
		&user.ProfilePhotoURL, &user.OnboardingCompleted, &user.CreatedAt,
		&user.UpdatedAt, &user.OnboardedAt,
	)
	return user, err
}

func linkOAuthIdentity(ctx context.Context, tx pgx.Tx, provider string, claims oidc.Claims, userID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO oauth_identities (user_id, provider, provider_user_id, provider_email)
		VALUES ($1, $2, $3, $4)`, userID, provider, claims.Subject, claims.Email)
	return err
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type googleTokenResponse struct {
	IDToken string `json:"id_token"`
	Error   string `json:"error"`
}

func exchangeGoogleCode(ctx context.Context, google *googleProvider, code, codeVerifier string) (string, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {google.clientID},
		"client_secret": {google.clientSecret},
		"redirect_uri":  {google.redirectURL},
		"grant_type":    {"authorization_code"},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call Google token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read Google token response: %w", err)
	}
	var parsed googleTokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse Google token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK || parsed.IDToken == "" {
		if parsed.Error != "" {
			return "", fmt.Errorf("Google token endpoint returned %q", parsed.Error)
		}
		return "", fmt.Errorf("Google token endpoint returned status %d", resp.StatusCode)
	}
	return parsed.IDToken, nil
}
