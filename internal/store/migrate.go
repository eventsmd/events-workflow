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
// (легаси-таблицы есть), делает baseline через Force: помечает миграции
// применёнными, не выполняя SQL. flyway_schema_history не трогается.
//
// Решение о baseline принимается по состоянию самого migrate-инстанса
// (m.Version()), а не по отдельному "до создания инстанса" запросу к
// schema_migrations. NewWithSourceInstance/Open уже идемпотентно создаёт
// schema_migrations (CREATE TABLE IF NOT EXISTS) как побочный эффект —
// если бы needBaseline решался ДО этого по "таблицы нет", то крэш между
// созданием инстанса и Force() оставлял бы пустую schema_migrations на
// проде: следующий старт видел бы таблицу, пропускал baseline и заново
// проигрывал 202512012250 (CREATE TABLE без IF NOT EXISTS) ⇒ dirty ⇒
// CrashLoop, требующий ручного SQL. m.Version() возвращает ErrNilVersion
// и для "таблицы нет", и для "таблица пустая" — оба случая корректно
// маппятся на "нужен baseline", если легаси-таблицы присутствуют.
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
