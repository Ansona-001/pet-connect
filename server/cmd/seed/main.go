// Command seed loads local/test-only demo data (see seed.sql). It refuses to
// run against any environment other than "local" or "test" — see ADR 0006
// (docs/adr/0006-migration-strategy.md) — so it can never accidentally
// populate staging or production with the demo accounts that used to live
// in migrations/000001_init.sql.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"petconnect/server/internal/config"
	"petconnect/server/internal/platform/database"
	"petconnect/server/migrations"
)

//go:embed seed.sql
var seedSQL string

var allowedEnvironments = map[string]bool{
	"local": true,
	"test":  true,
}

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if !allowedEnvironments[cfg.Environment] {
		return fmt.Errorf("refusing to seed demo data: APP_ENV=%q is not local or test", cfg.Environment)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	// Ensure the schema exists — the seed command is meant to be runnable on
	// its own, not only after `go run ./cmd/api` has started at least once.
	if err := migrations.Apply(ctx, db); err != nil {
		return fmt.Errorf("apply migrations before seeding: %w", err)
	}

	if _, err := db.Exec(ctx, seedSQL); err != nil {
		return fmt.Errorf("seed demo data: %w", err)
	}

	slog.Info("seeded local demo data", "environment", cfg.Environment)
	return nil
}
