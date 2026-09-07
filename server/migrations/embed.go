package migrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed *.sql
var files embed.FS

// advisoryLockKey is an arbitrary fixed key for the session-level advisory
// lock that serializes migration application across concurrent API
// replicas (ADR 0006, docs/adr/0006-migration-strategy.md). It only needs
// to be unique within this database; the value itself carries no meaning.
const advisoryLockKey = 8814_2601_0001

func Apply(ctx context.Context, pool *pgxpool.Pool) error {
	// Session-level advisory locks are held per connection, not per pool, so
	// every statement in this function must run on the same acquired
	// connection — a pool.Exec/QueryRow call can silently land on a
	// different connection and make the lock a no-op.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, int64(advisoryLockKey)); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		if _, err := conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, int64(advisoryLockKey)); err != nil {
			slog.Error("release migration lock", "error", err)
		}
	}()

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}

	entries, err := fs.Glob(files, "*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(entries)
	for _, name := range entries {
		var applied bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version=$1)`, name).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}

		sql, err := files.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, name); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}
