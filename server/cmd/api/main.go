package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/redis/go-redis/v9"

	"petconnect/server/internal/config"
	"petconnect/server/internal/modules/account"
	"petconnect/server/internal/modules/adoption"
	"petconnect/server/internal/modules/auth"
	"petconnect/server/internal/modules/chat"
	"petconnect/server/internal/modules/community"
	"petconnect/server/internal/modules/discovery"
	"petconnect/server/internal/modules/events"
	"petconnect/server/internal/modules/matching"
	"petconnect/server/internal/modules/media"
	"petconnect/server/internal/modules/notifications"
	"petconnect/server/internal/modules/pets"
	"petconnect/server/internal/modules/social"
	"petconnect/server/internal/platform/authjwt"
	"petconnect/server/internal/platform/database"
	"petconnect/server/internal/platform/httpx"
	"petconnect/server/internal/platform/storage"
	"petconnect/server/internal/realtime"
	"petconnect/server/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("petconnect API stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(rootCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := migrations.Apply(rootCtx, db); err != nil {
		return err
	}

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer redisClient.Close()
	if err := redisClient.Ping(rootCtx).Err(); err != nil {
		return errors.New("connect to Redis: " + err.Error())
	}

	mediaStore, err := storage.Open(
		rootCtx,
		cfg.S3Endpoint,
		cfg.S3AccessKey,
		cfg.S3SecretKey,
		cfg.S3Bucket,
		cfg.S3UseSSL,
	)
	if err != nil {
		return err
	}

	tokens := authjwt.New(cfg.AccessSecret, cfg.AccessTokenTTL)
	hub := realtime.NewHub(redisClient, "petconnect:realtime:events")
	if err := hub.Start(rootCtx); err != nil {
		return err
	}
	defer hub.Close()
	tickets := realtime.NewTicketStore(redisClient, "petconnect:realtime:ticket:", 30*time.Second)
	realtimeServer := realtime.NewServer(db, tickets, hub, tokens, cfg.AllowedOrigins)

	app := fiber.New(fiber.Config{
		AppName:               "PetConnect API",
		BodyLimit:             int(cfg.MaxUploadBytes),
		DisableStartupMessage: true,
		ReadTimeout:           30 * time.Second,
		WriteTimeout:          30 * time.Second,
		IdleTimeout:           90 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			var fiberError *fiber.Error
			if errors.As(err, &fiberError) {
				return httpx.Problem(c, fiberError.Code, "http_error", fiberError.Message)
			}
			slog.Error("unhandled request error", "request_id", c.GetRespHeader(fiber.HeaderXRequestID), "error", err)
			return httpx.Problem(c, fiber.StatusInternalServerError, "internal_error", "PetConnect could not complete the request.")
		},
	})
	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{
		AllowOriginsFunc: originMatcher(cfg.AllowedOrigins),
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, Idempotency-Key",
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		ExposeHeaders:    fiber.HeaderXRequestID,
		MaxAge:           600,
	}))
	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))
	app.Use(logger.New(logger.Config{
		Format:     "${time} ${status} ${latency} ${method} ${path} request_id=${respHeader:X-Request-ID}\n",
		TimeFormat: time.RFC3339,
	}))

	app.Get("/health/live", func(c *fiber.Ctx) error {
		return httpx.OK(c, fiber.Map{"status": "live"})
	})
	app.Get("/health/ready", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			return httpx.Problem(c, fiber.StatusServiceUnavailable, "database_unavailable", "PostgreSQL is unavailable.")
		}
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return httpx.Problem(c, fiber.StatusServiceUnavailable, "redis_unavailable", "Redis is unavailable.")
		}
		return httpx.OK(c, fiber.Map{"status": "ready"})
	})
	app.Static("/media/demo", cfg.DemoAssetDir, fiber.Static{MaxAge: 86400})

	v1 := app.Group("/v1")
	auth.RegisterRoutes(v1, db, tokens, cfg.RefreshTokenTTL)
	// The socket route authenticates with a single-use ticket, so mount it
	// before the bearer middleware that protects the rest of /v1.
	realtimeServer.RegisterSocketRoute(v1)
	protected := v1.Group("", httpx.Authenticate(tokens))
	account.RegisterRoutes(protected, db)
	pets.RegisterRoutes(protected, db)
	social.RegisterRoutes(protected, db)
	discovery.RegisterRoutes(protected, db)
	community.RegisterRoutes(protected, db)
	events.RegisterRoutes(protected, db)
	adoption.RegisterRoutes(protected, db)
	notifications.RegisterRoutes(protected, db)

	matchingService := matching.NewService(db, hub)
	matching.RegisterRoutes(protected, matchingService)
	chatService := chat.NewService(db, hub)
	chat.RegisterRoutes(protected, chatService)
	media.New(mediaStore, cfg.PublicBaseURL, cfg.MaxUploadBytes).Register(protected, app)
	realtimeServer.RegisterTicketRoute(protected)

	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("PetConnect API listening", "address", cfg.HTTPAddr, "environment", cfg.Environment)
		serverErrors <- app.Listen(cfg.HTTPAddr)
	}()

	select {
	case <-rootCtx.Done():
		shutdownDone := make(chan error, 1)
		go func() { shutdownDone <- app.Shutdown() }()
		select {
		case err := <-shutdownDone:
			return err
		case <-time.After(10 * time.Second):
			return errors.New("HTTP server shutdown timed out")
		}
	case err := <-serverErrors:
		return err
	}
}

func originMatcher(patterns []string) func(string) bool {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		quoted := regexp.QuoteMeta(strings.TrimSpace(pattern))
		quoted = strings.ReplaceAll(quoted, `\*`, `[^/]*`)
		if expression, err := regexp.Compile("^" + quoted + "$"); err == nil {
			compiled = append(compiled, expression)
		}
	}
	return func(origin string) bool {
		for _, expression := range compiled {
			if expression.MatchString(origin) {
				return true
			}
		}
		return false
	}
}
