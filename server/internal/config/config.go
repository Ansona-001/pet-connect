package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment     string
	HTTPAddr        string
	DatabaseURL     string
	RedisAddr       string
	S3Endpoint      string
	S3AccessKey     string
	S3SecretKey     string
	S3Bucket        string
	S3UseSSL        bool
	AccessSecret    string
	RefreshSecret   string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	AllowedOrigins  []string
	DemoAssetDir    string
	PublicBaseURL   string
	MaxUploadBytes  int64

	// Google Sign-In (ADR 0005). All three deliberately default to empty:
	// the owner's Google Cloud project/client credentials are a genuinely
	// blocking external input (brief §18) this repo cannot invent. Their
	// absence is exactly what keeps the feature disabled — see
	// auth.googleOAuthConfigured — rather than a separate on/off flag that
	// could drift out of sync with whether real credentials exist.
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// Sign in with Apple (ADR 0005). Apple's OAuth client "secret" isn't a
	// static value like Google's — it's a short-lived ES256 JWT the server
	// mints per request from the owner's Apple Developer private key (see
	// auth.appleProvider.clientSecret), so what's configured here is the
	// raw material for that (Services ID, Team ID, Key ID, and the .p8
	// private key contents) rather than a client secret directly. All five
	// default to empty for the same reason as Google's: a real Apple
	// Developer account/team is a blocking external input this repo
	// cannot invent, and its absence is what keeps the feature disabled.
	AppleClientID    string
	AppleTeamID      string
	AppleKeyID       string
	ApplePrivateKey  string
	AppleRedirectURL string
}

func Load() (Config, error) {
	accessTTL, err := time.ParseDuration(env("ACCESS_TOKEN_TTL", "15m"))
	if err != nil {
		return Config{}, fmt.Errorf("parse ACCESS_TOKEN_TTL: %w", err)
	}
	refreshTTL, err := time.ParseDuration(env("REFRESH_TOKEN_TTL", "720h"))
	if err != nil {
		return Config{}, fmt.Errorf("parse REFRESH_TOKEN_TTL: %w", err)
	}
	useSSL, err := strconv.ParseBool(env("S3_USE_SSL", "false"))
	if err != nil {
		return Config{}, fmt.Errorf("parse S3_USE_SSL: %w", err)
	}
	maxUpload, err := strconv.ParseInt(env("MAX_UPLOAD_BYTES", "52428800"), 10, 64)
	if err != nil {
		return Config{}, fmt.Errorf("parse MAX_UPLOAD_BYTES: %w", err)
	}

	cfg := Config{
		Environment:     env("APP_ENV", "local"),
		HTTPAddr:        env("HTTP_ADDR", ":8080"),
		DatabaseURL:     env("DATABASE_URL", "postgres://petconnect:petconnect_dev@127.0.0.1:5433/petconnect?sslmode=disable"),
		RedisAddr:       env("REDIS_ADDR", "127.0.0.1:6380"),
		S3Endpoint:      env("S3_ENDPOINT", "127.0.0.1:9000"),
		S3AccessKey:     env("S3_ACCESS_KEY", "petconnect"),
		S3SecretKey:     env("S3_SECRET_KEY", "petconnect_dev_secret"),
		S3Bucket:        env("S3_BUCKET", "petconnect-media"),
		S3UseSSL:        useSSL,
		AccessSecret:    env("JWT_ACCESS_SECRET", "change-this-access-secret-before-production"),
		RefreshSecret:   env("JWT_REFRESH_SECRET", "change-this-refresh-secret-before-production"),
		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
		AllowedOrigins:  split(env("ALLOWED_ORIGINS", "http://localhost:*;http://127.0.0.1:*")),
		DemoAssetDir:    env("DEMO_ASSET_DIR", "../assets/images"),
		PublicBaseURL:   strings.TrimRight(env("PUBLIC_BASE_URL", "http://localhost:8080"), "/"),
		MaxUploadBytes:  maxUpload,

		GoogleClientID:     env("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: env("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  env("GOOGLE_REDIRECT_URL", ""),

		AppleClientID:    env("APPLE_CLIENT_ID", ""),
		AppleTeamID:      env("APPLE_TEAM_ID", ""),
		AppleKeyID:       env("APPLE_KEY_ID", ""),
		ApplePrivateKey:  normalizePEM(env("APPLE_PRIVATE_KEY", "")),
		AppleRedirectURL: env("APPLE_REDIRECT_URL", ""),
	}

	if cfg.Environment != "local" {
		if strings.HasPrefix(cfg.AccessSecret, "change-this") || strings.HasPrefix(cfg.RefreshSecret, "change-this") {
			return Config{}, fmt.Errorf("production JWT secrets must be configured")
		}
		if len(cfg.AccessSecret) < 32 || len(cfg.RefreshSecret) < 32 {
			return Config{}, fmt.Errorf("JWT secrets must contain at least 32 bytes")
		}
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// normalizePEM lets APPLE_PRIVATE_KEY be supplied either as a real
// multi-line value (most .env loaders and orchestrators support this) or
// as a single-line value with literal `\n` escapes (common when a secret
// is passed through a shell variable or a CI secret store that flattens
// newlines) — x509 parsing requires real newlines, so a single-line value
// is unescaped; a value that already contains real newlines is left
// untouched.
func normalizePEM(value string) string {
	if value == "" || strings.Contains(value, "\n") {
		return value
	}
	return strings.ReplaceAll(value, `\n`, "\n")
}

func split(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ';' || r == ',' })
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
