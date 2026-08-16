package store

import (
	"context"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"events-workflow/internal/domain"
)

func startPostgres(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("events"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testcontainers.TerminateContainer(pg) })
	url, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	return url
}

func TestMigrateAndRepos(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	url := startPostgres(t)

	if err := Migrate(url); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(url); err != nil { // идемпотентность
		t.Fatal(err)
	}

	pool, err := NewPool(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s := New(pool)

	// --- telegram_messages: upsert + hstore + roundtrip
	inc := uuid.New()
	e := TelegramMessageEntityFrom(domain.TelegramMessage{
		ID: 1, ChatID: 2, Text: "hello",
		Date:    domain.LocalDateTime{Time: time.Date(2025, 12, 1, 22, 50, 0, 0, time.UTC)},
		From:    &domain.User{ID: 3, Name: "n"},
		Context: map[string]string{"supplier": "water"},
	}, inc)
	if err := s.SaveMessage(ctx, e); err != nil {
		t.Fatal(err)
	}
	e.Text = "updated"
	if err := s.SaveMessage(ctx, e); err != nil { // saveAndFlush = merge
		t.Fatal(err)
	}
	got, err := s.FindMessage(ctx, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Text != "updated" || *got.IncidentID != inc ||
		got.Context["supplier"] != "water" {
		t.Fatalf("%+v", got)
	}
	if missing, _ := s.FindMessage(ctx, 404, 404); missing != nil {
		t.Fatal("expected nil for missing row")
	}

	// --- transcribes
	start := time.Date(2025, 12, 2, 9, 0, 0, 0, time.UTC)
	tr := &TranscribeEntity{ID: 1, ChatID: 2, Event: StrPtr("shutdown"), EventStart: &start}
	if err := s.SaveTranscribe(ctx, tr); err != nil {
		t.Fatal(err)
	}
	gotTr, err := s.FindTranscribe(ctx, 1, 2)
	if err != nil || gotTr == nil || *gotTr.Event != "shutdown" || !gotTr.EventStart.Equal(start) {
		t.Fatalf("%+v %v", gotTr, err)
	}

	// --- addresses
	a := &AddressEntity{ID: uuid.New(), MessageID: 1, ChatID: 2,
		CityOriginal: StrPtr("Тирасполь"), StreetKladr: StrPtr("123-01.001-02.002-00.000-04.004")}
	if err := s.SaveAddresses(ctx, []*AddressEntity{a}); err != nil {
		t.Fatal(err)
	}
	byID, err := s.FindAddress(ctx, a.ID)
	if err != nil || byID == nil || *byID.CityOriginal != "Тирасполь" {
		t.Fatalf("%+v %v", byID, err)
	}
	byID.CityName = StrPtr("Тирасполь")
	if err := s.SaveAddress(ctx, byID); err != nil {
		t.Fatal(err)
	}
	list, err := s.FindAddressesByMessage(ctx, 1, 2)
	if err != nil || len(list) != 1 || *list[0].CityName != "Тирасполь" {
		t.Fatalf("%v %v", list, err)
	}

	// --- subscriptions: prefix match
	sub := &Subscription{ID: uuid.New(), CreatedAt: time.Now(),
		SubscribeToKladr: "123-01.001-02.002-00.000-04.004", TgID: "777",
		SubscribeToFulltext: "г. Тирасполь, ул. Ленина"}
	if err := s.SaveSubscription(ctx, sub); err != nil {
		t.Fatal(err)
	}
	subs, err := s.FindSubscriptionsByKladrPrefix(ctx, "123-01.001-02.002")
	if err != nil || len(subs) != 1 || subs[0].TgID != "777" {
		t.Fatalf("%v %v", subs, err)
	}
	subs, _ = s.FindSubscriptionsByKladrPrefix(ctx, "999")
	if len(subs) != 0 {
		t.Fatal("expected no match")
	}
}

func TestMigrate_BaselineExistingFlywayDB(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	url := startPostgres(t)
	pool, err := NewPoolNoHstore(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	// Эмулируем прод-базу, мигрированную Flyway: таблицы есть, schema_migrations нет.
	for _, f := range []string{
		"migrations/202512012250_create_telegram_messages.up.sql",
		"migrations/202512030730_create_subscriptions_table.up.sql",
	} {
		sqlBytes, err := migrationsFS.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatal(err)
		}
	}
	pool.Close()

	if err := Migrate(url); err != nil { // не должен пытаться выполнить CREATE TABLE повторно
		t.Fatal(err)
	}
}

// TestMigrate_BaselineAfterCrashWindow эмулирует крэш ровно между
// созданием schema_migrations (побочный эффект NewWithSourceInstance) и
// вызовом Force(baselineVersion): таблица schema_migrations существует,
// но пустая, а легаси-таблицы Flyway уже есть. Следующий вызов Migrate
// должен по-прежнему сделать baseline, а не попытаться заново применить
// 202512012250 (CREATE TABLE без IF NOT EXISTS упал бы, оставив базу
// dirty).
func TestMigrate_BaselineAfterCrashWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	url := startPostgres(t)

	// Шаг 1: как будто Migrate() уже создал migrate-инстанс (что создаёт
	// пустую schema_migrations), но процесс упал до Force().
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, url)
	if err != nil {
		t.Fatal(err)
	}
	if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
		t.Fatalf("close: src=%v db=%v", srcErr, dbErr)
	}

	// Шаг 2: легаси-таблицы Flyway-прода уже существуют.
	pool, err := NewPoolNoHstore(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{
		"migrations/202512012250_create_telegram_messages.up.sql",
		"migrations/202512030730_create_subscriptions_table.up.sql",
	} {
		sqlBytes, err := migrationsFS.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatal(err)
		}
	}
	pool.Close()

	if err := Migrate(url); err != nil {
		t.Fatalf("expected baseline to recover from crash window, got: %v", err)
	}
}
