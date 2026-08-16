package store

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// baselineVersion — версия последней миграции, применённой Flyway в проде.
const baselineVersion = 202512030730

// Migrate выполняет миграции. Для базы, ранее мигрированной Flyway
// (таблицы есть, schema_migrations нет), делает baseline через Force:
// помечает миграции применёнными, не выполняя SQL. flyway_schema_history
// не трогается.
func Migrate(pgURL string) error {
	needBaseline, err := flywayBaselineNeeded(pgURL)
	if err != nil {
		return err
	}

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migrations source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, pgURL)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if needBaseline {
		if err := m.Force(baselineVersion); err != nil {
			return fmt.Errorf("baseline: %w", err)
		}
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func flywayBaselineNeeded(pgURL string) (bool, error) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, pgURL)
	if err != nil {
		return false, fmt.Errorf("connect for baseline check: %w", err)
	}
	defer conn.Close(ctx)
	var hasMigrations, hasLegacy bool
	err = conn.QueryRow(ctx,
		`SELECT to_regclass('schema_migrations') IS NOT NULL,
		        to_regclass('telegram_messages') IS NOT NULL`).
		Scan(&hasMigrations, &hasLegacy)
	if err != nil {
		return false, err
	}
	return !hasMigrations && hasLegacy, nil
}
