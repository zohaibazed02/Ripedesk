package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	direction := flag.String("direction", "up", "Migration direction: up or down")
	migrationsDir := flag.String("dir", "./migrations", "Path to SQL migrations directory")
	steps := flag.Int("steps", 0, "Number of migrations to apply/rollback (0 = all)")
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://hospitality:hospitality_dev_secret@localhost:5432/hospitality_cloud?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer conn.Close(ctx)

	if err := ensureMigrationsTable(ctx, conn); err != nil {
		log.Fatalf("ensure schema_migrations table: %v", err)
	}

	switch *direction {
	case "up":
		if err := migrateUp(ctx, conn, *migrationsDir, *steps); err != nil {
			log.Fatalf("migrate up failed: %v", err)
		}
	case "down":
		if err := migrateDown(ctx, conn, *migrationsDir, *steps); err != nil {
			log.Fatalf("migrate down failed: %v", err)
		}
	default:
		log.Fatalf("unsupported direction %q (use up or down)", *direction)
	}
}

func ensureMigrationsTable(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     TEXT PRIMARY KEY,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func listMigrationFiles(dir, suffix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), suffix) {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func versionFromFilename(filename, suffix string) string {
	return strings.TrimSuffix(filename, suffix)
}

func migrateUp(ctx context.Context, conn *pgx.Conn, dir string, steps int) error {
	files, err := listMigrationFiles(dir, ".up.sql")
	if err != nil {
		return err
	}

	applied := 0
	for _, file := range files {
		if steps > 0 && applied >= steps {
			break
		}
		version := versionFromFilename(file, ".up.sql")
		var exists bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}

		sqlBytes, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			return err
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("apply %s: %w", file, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		log.Printf("applied migration: %s", version)
		applied++
	}

	log.Printf("migration up complete (%d applied)", applied)
	return nil
}

func migrateDown(ctx context.Context, conn *pgx.Conn, dir string, steps int) error {
	files, err := listMigrationFiles(dir, ".down.sql")
	if err != nil {
		return err
	}
	// Roll back in reverse order
	for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
		files[i], files[j] = files[j], files[i]
	}

	rolledBack := 0
	for _, file := range files {
		if steps > 0 && rolledBack >= steps {
			break
		}
		version := versionFromFilename(file, ".down.sql")
		var exists bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			continue
		}

		sqlBytes, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			return err
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("rollback %s: %w", file, err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		log.Printf("rolled back migration: %s", version)
		rolledBack++
	}

	log.Printf("migration down complete (%d rolled back)", rolledBack)
	return nil
}
