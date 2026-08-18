package store

import (
	"context"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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
	if err := Migrate(url); err != nil { // idempotency
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
	if err := s.SaveMessage(ctx, e); err != nil { // SaveMessage = merge (upsert)
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
	// Emulate production database migrated by Flyway: tables exist, schema_migrations does not.
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

	if err := Migrate(url); err != nil { // must not attempt to execute CREATE TABLE again
		t.Fatal(err)
	}
}

// TestMigrate_BaselineAfterCrashWindow emulates a crash exactly between
// creation of schema_migrations (side effect of NewWithSourceInstance) and
// the Force(baselineVersion) call: the schema_migrations table exists,
// but is empty, and Flyway legacy tables are already present. The next
// Migrate call must still perform baseline, not attempt to replay
// 202512012250 (CREATE TABLE without IF NOT EXISTS would fail, leaving the database
// dirty).
func TestMigrate_BaselineAfterCrashWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	url := startPostgres(t)

	// Step 1: as if Migrate() already created a migrate instance (which creates
	// an empty schema_migrations), but the process crashed before Force().
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

	// Step 2: Flyway legacy tables from production already exist.
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

// TestMigrate_BaselineOnlyTelegramMessages_AppliesSubscriptions covers a
// database whose Flyway history stopped at the first migration (older
// dump, DB created before the subscriptions migration was added, or a
// failed second script): telegram_messages exists but subscriptions does
// not, and schema_migrations does not exist yet either. Baselining
// straight to baselineVersion (as if both tables existed) would mark the
// subscriptions migration applied without ever creating its table, so
// every Notify would fail forever with "relation \"subscriptions\" does
// not exist". Migrate must instead baseline to the lower version so Up()
// goes on to actually create subscriptions.
func TestMigrate_BaselineOnlyTelegramMessages_AppliesSubscriptions(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	url := startPostgres(t)
	pool, err := NewPoolNoHstore(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	sqlBytes, err := migrationsFS.ReadFile("migrations/202512012250_create_telegram_messages.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatal(err)
	}
	pool.Close()

	if err := Migrate(url); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	pool2, err := NewPoolNoHstore(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool2.Close()
	var hasSubscriptions bool
	if err := pool2.QueryRow(ctx, `SELECT to_regclass('subscriptions') IS NOT NULL`).Scan(&hasSubscriptions); err != nil {
		t.Fatal(err)
	}
	if !hasSubscriptions {
		t.Fatal("expected subscriptions table to exist after Migrate")
	}

	// A subsequent Migrate call must be a no-op (idempotent), not error.
	if err := Migrate(url); err != nil {
		t.Fatalf("second Migrate call: %v", err)
	}
}

// TestStore_WithConn_RespectsCallerDeadline — regression test for pool
// starvation blocking an activity for the full 5-minute
// StartToCloseTimeout (see acquireTimeout in store.go): with the pool's
// single connection held elsewhere, a Store call must fail fast once its
// context expires rather than hang.
func TestStore_WithConn_RespectsCallerDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	url := startPostgres(t)
	if err := Migrate(url); err != nil {
		t.Fatal(err)
	}

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	held, err := pool.Acquire(ctx) // exhaust the pool's only connection
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	s := New(pool)
	shortCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = s.FindMessage(shortCtx, 1, 2)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error: pool exhausted")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("FindMessage blocked for %v instead of respecting the caller's deadline", elapsed)
	}
}
