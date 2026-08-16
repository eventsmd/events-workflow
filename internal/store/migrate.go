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

// baselineVersion — version of the latest migration applied by Flyway in production.
const baselineVersion = 202512030730

// Migrate executes migrations. For a database previously migrated by Flyway
// (legacy tables exist), it performs baseline via Force: marks migrations
// as applied without executing SQL. flyway_schema_history is not touched.
//
// The decision about baseline is made based on the state of the migrate instance
// (m.Version()), not via a separate query to schema_migrations before instance
// creation. NewWithSourceInstance/Open already idempotently creates
// schema_migrations (CREATE TABLE IF NOT EXISTS) as a side effect —
// if needBaseline was decided BEFORE this by "table does not exist", a crash between
// instance creation and Force() would leave an empty schema_migrations in
// production: the next start would see the table, skip baseline, and replay
// 202512012250 (CREATE TABLE without IF NOT EXISTS) ⇒ dirty ⇒
// CrashLoop, requiring manual SQL. m.Version() returns ErrNilVersion
// both for "table does not exist" and "table is empty" — both cases correctly
// map to "baseline is needed" if legacy tables are present.
func Migrate(pgURL string) error {
	hasLegacy, err := legacyTablesPresent(pgURL)
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

	_, _, verErr := m.Version()
	if verErr != nil && !errors.Is(verErr, migrate.ErrNilVersion) {
		return fmt.Errorf("migrate version: %w", verErr)
	}
	needBaseline := hasLegacy && errors.Is(verErr, migrate.ErrNilVersion)

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

func legacyTablesPresent(pgURL string) (bool, error) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, pgURL)
	if err != nil {
		return false, fmt.Errorf("connect for baseline check: %w", err)
	}
	defer conn.Close(ctx)
	var hasLegacy bool
	err = conn.QueryRow(ctx,
		`SELECT to_regclass('telegram_messages') IS NOT NULL`).Scan(&hasLegacy)
	if err != nil {
		return false, err
	}
	return hasLegacy, nil
}
