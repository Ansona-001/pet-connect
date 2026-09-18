package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"

	"petconnect/server/internal/platform/httpx"
	"petconnect/server/internal/platform/oidc"
)

const (
	appleAuthorizationEndpoint = "https://appleid.apple.com/auth/authorize"
	appleTokenEndpoint         = "https://appleid.apple.com/auth/token"
	appleJWKSURL               = "https://appleid.apple.com/auth/keys"
	appleIssuer                = "https://appleid.apple.com"

	// appleClientSecretTTL is deliberately short (Apple allows up to six
	// months): the secret is minted fresh for each token exchange rather
	// than cached, so there's no benefit to a long lifetime and a short
	// one limits how long a leaked value would matter.
	appleClientSecretTTL = 5 * time.Minute
)

// AppleOAuthConfig carries the raw material for Sign in with Apple — see
// config.Config's doc comment for why every field defaults to empty.
// Unlike Google, there is no single static "client secret" from Apple:
// TeamID/KeyID/PrivateKey are instead the ingredients appleProvider.
// clientSecret uses to mint one per request.
type AppleOAuthConfig struct {
	ClientID    string // the Services ID registered with Sign in with Apple
	TeamID      string
	KeyID       string
	PrivateKey  string // PEM-encoded PKCS8 EC private key (the .p8 file's contents)
	RedirectURL string
}

// appleProvider is non-nil only once every AppleOAuthConfig field is
// present *and* the private key parses — see newAppleProvider. Its
// presence/absence is the on/off switch for the feature, same as
// googleProvider for Google.
type appleProvider struct {
	clientID    string
	teamID      string
	keyID       string
	privateKey  *ecdsa.PrivateKey
	redirectURL string
	verifier    *oidc.Verifier
}

// newAppleProvider fails closed: a missing field leaves Apple disabled
// silently (the expected local/CI state), but a *present* private key
// that fails to parse is logged rather than left silently broken, since
// that's much more likely to be the owner's own misconfiguration (wrong
// file, truncated copy-paste, PKCS1 instead of PKCS8) worth surfacing.
func newAppleProvider(cfg AppleOAuthConfig) *appleProvider {
	if cfg.ClientID == "" || cfg.TeamID == "" || cfg.KeyID == "" || cfg.PrivateKey == "" || cfg.RedirectURL == "" {
		return nil
	}
	key, err := parseApplePrivateKey(cfg.PrivateKey)
	if err != nil {
		slog.Error("parse APPLE_PRIVATE_KEY; Sign in with Apple stays disabled", "error", err)
		return nil
	}
	return &appleProvider{
		clientID:    cfg.ClientID,
		teamID:      cfg.TeamID,
		keyID:       cfg.KeyID,
		privateKey:  key,
		redirectURL: cfg.RedirectURL,
		verifier:    oidc.NewVerifier(appleJWKSURL, cfg.ClientID, appleIssuer),
	}
}

func parseApplePrivateKey(pemContents string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemContents))
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an ECDSA key")
	}
	return key, nil
}

// clientSecret mints the short-lived ES256 JWT Apple requires in place
// of a static client secret (Apple's token endpoint verifies it the same
// way it would any client_secret, but it must be freshly signed with the
// developer's private key rather than a value the server can just store).
func (a *appleProvider) clientSecret(now time.Time) (string, error) {
	claims := jwt.RegisteredClaims{
		Issuer:    a.teamID,
		Subject:   a.clientID,
		Audience:  jwt.ClaimStrings{appleIssuer},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(appleClientSecretTTL)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = a.keyID
	signed, err := token.SignedString(a.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign Apple client secret: %w", err)
	}
	return signed, nil
}

// appleStart mirrors googleStart — see its comment for why this returns
// JSON rather than issuing a redirect. code_challenge/code_challenge_method
// are optional for Apple (unlike Google, where PKCE is required for this
// flow) but included for the same defense-in-depth reasoning; Apple
// simply accepts them.
func (h *Handler) appleStart(c *fiber.Ctx) error {
	if h.apple == nil {
		return httpx.Problem(c, fiber.StatusServiceUnavailable, "apple_oauth_not_configured", "Sign in with Apple is not configured on this server.")
	}

	state, verifier, err := beginOAuthState(c.UserContext(), h.redis)
	if err != nil {
		return internalProblem(c)
	}
	challenge := pkceChallenge(verifier)

	query := url.Values{
		"client_id":     {h.apple.clientID},
		"redirect_uri":  {h.apple.redirectURL},
		"response_type": {"code"},
		"scope":         {"name email"},
		// Apple requires form_post whenever the requested scope includes
		// name or email — see appleCallback, which is a POST route.
		"response_mode":         {"form_post"},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}

	c.Set(fiber.HeaderCacheControl, "no-store")
	return httpx.OK(c, authorizationURLResponse{
		AuthorizationURL: appleAuthorizationEndpoint + "?" + query.Encode(),
	})
}

// appleUserParam is the shape of the one-time `user` form field Apple
// sends alongside the very first authorization for a given account —
// subsequent sign-ins omit it entirely, which is why claims.Name is
// filled in here rather than being expected from the ID token (Apple's
// ID tokens never carry a name claim at all, unlike Google's).
type appleUserParam struct {
	Name struct {
		FirstName string `json:"firstName"`
		LastName  string `json:"lastName"`
	} `json:"name"`
}

func appleNameFromUserParam(raw string) string {
	if raw == "" {
		return ""
	}
	var parsed appleUserParam
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Name.FirstName + " " + parsed.Name.LastName)
}

// appleCallback is where Apple redirects (via a POST, per response_mode
// above) after the user consents or declines. See googleCallback for the
// shared reasoning behind the exchange-ticket deep-link handoff — the
// same "endpoint works, the app-side deep link receiver doesn't exist
// yet" boundary applies here too.
func (h *Handler) appleCallback(c *fiber.Ctx) error {
	if h.apple == nil {
		return httpx.Problem(c, fiber.StatusServiceUnavailable, "apple_oauth_not_configured", "Sign in with Apple is not configured on this server.")
	}

	if oauthErr := c.FormValue("error"); oauthErr != "" {
		return httpx.Problem(c, fiber.StatusBadRequest, "apple_oauth_denied", "Apple sign-in was cancelled or denied.")
	}
	code := strings.TrimSpace(c.FormValue("code"))
	state := strings.TrimSpace(c.FormValue("state"))
	if code == "" || state == "" {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_oauth_callback", "The OAuth callback is missing required parameters.")
	}

	verifier, err := redeemOAuthState(c.UserContext(), h.redis, state)
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadRequest, "invalid_oauth_state", "This sign-in attempt has expired or was already used. Please try again.")
	}

	clientSecret, err := h.apple.clientSecret(h.clock().UTC())
	if err != nil {
		return internalProblem(c)
	}
	idToken, err := exchangeAppleCode(c.UserContext(), h.apple, code, verifier, clientSecret)
	if err != nil {
		return httpx.Problem(c, fiber.StatusBadGateway, "apple_token_exchange_failed", "Could not complete Apple sign-in.")
	}

	claims, err := h.apple.verifier.Verify(c.UserContext(), idToken)
	if err != nil {
		return httpx.Problem(c, fiber.StatusUnauthorized, "invalid_apple_id_token", "Could not verify the Apple identity token.")
	}
	if name := appleNameFromUserParam(c.FormValue("user")); name != "" {
		claims.Name = name
	}

	result, err := h.linkOrCreateOAuthAccount(c.UserContext(), "apple", claims, c.Get(fiber.HeaderUserAgent))
	if err != nil {
		if errors.Is(err, errOAuthEmailInUseUnverified) {
			return httpx.Problem(c, fiber.StatusConflict, "email_in_use", "An account already exists for that email address. Sign in with your password instead.")
		}
		return internalProblem(c)
	}

	ticket, err := h.mintExchangeTicket(c.UserContext(), result)
	if err != nil {
		return internalProblem(c)
	}

	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Redirect("petconnect://auth/callback?ticket=" + url.QueryEscape(ticket))
}

type appleTokenResponse struct {
	IDToken string `json:"id_token"`
	Error   string `json:"error"`
}

func exchangeAppleCode(ctx context.Context, apple *appleProvider, code, codeVerifier, clientSecret string) (string, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {apple.clientID},
		"client_secret": {clientSecret},
		"redirect_uri":  {apple.redirectURL},
		"grant_type":    {"authorization_code"},
		"code_verifier": {codeVerifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, appleTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call Apple token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read Apple token response: %w", err)
	}
	var parsed appleTokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parse Apple token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK || parsed.IDToken == "" {
		if parsed.Error != "" {
			return "", fmt.Errorf("Apple token endpoint returned %q", parsed.Error)
		}
		return "", fmt.Errorf("Apple token endpoint returned status %d", resp.StatusCode)
	}
	return parsed.IDToken, nil
}
