package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// baselineVersion — version of the latest migration applied by Flyway in production.
const baselineVersion = 202512030730

// telegramMessagesVersion — version of the migration that only creates
// telegram_messages, i.e. the state of a database that stopped at the
// first Flyway script (older dump, DB created before the subscriptions
// migration was added, or a failed second script).
const telegramMessagesVersion = 202512012250

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
	legacyVersion, err := legacyBaselineVersion(pgURL)
	if err != nil {
		return err
	}

	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migrations source: %w", err)
	}
	migrateURL, err := pgx5MigrateURL(pgURL)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, migrateURL)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	_, _, verErr := m.Version()
	if verErr != nil && !errors.Is(verErr, migrate.ErrNilVersion) {
		return fmt.Errorf("migrate version: %w", verErr)
	}
	needBaseline := legacyVersion != 0 && errors.Is(verErr, migrate.ErrNilVersion)

	if needBaseline {
		if err := m.Force(legacyVersion); err != nil {
			return fmt.Errorf("baseline: %w", err)
		}
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// pgx5MigrateURL rewrites the scheme of a postgres/postgresql URL to pgx5,
// selecting golang-migrate's pgx-based driver instead of its default
// lib/pq-based one. lib/pq only accepts sslmode values of "require"
// (default), "verify-full", "verify-ca", and "disable" — it rejects
// "prefer" outright — while pgx (used for the runtime pool too) supports
// "prefer" and defaults to it, matching pgjdbc's default. Using the pgx5
// driver here keeps migrate's SSL behavior consistent with the rest of the
// service instead of forcing a value lib/pq can't accept.
func pgx5MigrateURL(pgURL string) (string, error) {
	u, err := url.Parse(pgURL)
	if err != nil {
		return "", fmt.Errorf("parse database URL: %w", err)
	}
	u.Scheme = "pgx5"
	return u.String(), nil
}

// legacyBaselineVersion probes which Flyway-era tables actually exist and
// returns the version to Force() the baseline to, so it always matches the
// newest migration whose objects are really present — never higher.
// A database whose Flyway history stopped at telegramMessagesVersion (older
// dump, DB created before the subscriptions migration was added, or a
// failed second script) has telegram_messages but not subscriptions:
// baselining straight to baselineVersion would mark the subscriptions
// migration applied even though its table doesn't exist, so Up() would
// return ErrNoChange and every subsequent query against subscriptions
// would fail with "relation \"subscriptions\" does not exist", forever.
// Returns 0 when neither legacy table exists (no baseline needed).
func legacyBaselineVersion(pgURL string) (int, error) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, pgURL)
	if err != nil {
		return 0, fmt.Errorf("connect for baseline check: %w", err)
	}
	defer conn.Close(ctx)
	var hasTelegramMessages, hasSubscriptions bool
	err = conn.QueryRow(ctx,
		`SELECT to_regclass('telegram_messages') IS NOT NULL,
		        to_regclass('subscriptions') IS NOT NULL`).
		Scan(&hasTelegramMessages, &hasSubscriptions)
	if err != nil {
		return 0, err
	}
	switch {
	case hasTelegramMessages && hasSubscriptions:
		return baselineVersion, nil
	case hasTelegramMessages:
		return telegramMessagesVersion, nil
	default:
		return 0, nil
	}
}
