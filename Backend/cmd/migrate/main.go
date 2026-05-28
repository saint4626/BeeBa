package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"beeba.org/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		log.Fatal("BEEBA_DATABASE_URL is required to run migrations")
	}

	migrationsDir := os.Getenv("BEEBA_MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "./migrations"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := waitForDatabase(ctx, db); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	if err := ensureSchemaMigrations(ctx, db); err != nil {
		log.Fatalf("ensure schema migrations: %v", err)
	}

	files, err := migrationFiles(migrationsDir)
	if err != nil {
		log.Fatalf("list migrations: %v", err)
	}

	for _, file := range files {
		if err := applyMigration(ctx, db, file); err != nil {
			log.Fatalf("apply migration %s: %v", filepath.Base(file), err)
		}
	}

	log.Printf("migrations complete: %d checked", len(files))
}

func waitForDatabase(ctx context.Context, db *sql.DB) error {
	const retryInterval = 2 * time.Second

	var lastErr error
	attempt := 1
	for {
		if err := db.PingContext(ctx); err == nil {
			if attempt > 1 {
				log.Printf("database reachable after %d attempts", attempt)
			}
			return nil
		} else {
			lastErr = err
			log.Printf("database not ready, retrying in %s (attempt %d): %v", retryInterval, attempt, err)
		}

		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			if lastErr != nil {
				return fmt.Errorf("%w: last ping error: %v", ctx.Err(), lastErr)
			}
			return ctx.Err()
		case <-timer.C:
			attempt++
		}
	}
}

func ensureSchemaMigrations(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version text PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
)`)
	return err
}

func migrationFiles(dir string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	return matches, nil
}

func applyMigration(ctx context.Context, db *sql.DB, file string) error {
	version := strings.TrimSuffix(filepath.Base(file), ".up.sql")
	applied, err := migrationApplied(ctx, db, version)
	if err != nil {
		return err
	}
	if applied {
		log.Printf("skip migration %s", version)
		return nil
	}

	sqlBytes, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	log.Printf("applied migration %s", version)
	return nil
}

func migrationApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	var applied bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&applied)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query schema_migrations: %w", err)
	}
	return applied, nil
}
