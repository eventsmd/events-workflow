package workflows

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"events-workflow/internal/domain"
	"events-workflow/internal/store"
)

// startPostgres — same as store.startPostgres (not exported,
// so minimally duplicated here).
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

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	url := startPostgres(t)
	if err := store.Migrate(url); err != nil {
		t.Fatal(err)
	}
	pool, err := store.NewPool(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return store.New(pool)
}

// TestNotify_NoMatchingSubscriptions_NilEventStart_NoError — Finding 1
// regression: an address whose KLADR prefix matches no subscription must
// not fail the activity even though transcribe.EventStart is nil. Java only
// touches messageTranscribe.getEventStart() inside the per-subscription
// forEach, so a zero-subscriber address never reaches that code at all.
func TestNotify_NoMatchingSubscriptions_NilEventStart_NoError(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a := &Activities{Store: s}

	const id, chatID = int64(1), int64(-100)
	msg := domain.TelegramMessage{
		ID: id, ChatID: chatID, Text: "hello",
		Date:    domain.LocalDateTime{Time: time.Now()},
		From:    &domain.User{ID: 1, Name: "n"},
		Context: map[string]string{"supplier": "water"},
	}
	if err := s.SaveMessage(ctx, store.TelegramMessageEntityFrom(msg, uuid.New())); err != nil {
		t.Fatal(err)
	}
	// EventStart intentionally nil — must never be dereferenced when no
	// subscription matches.
	if err := s.SaveTranscribe(ctx, &store.TranscribeEntity{
		ID: id, ChatID: chatID, Event: store.StrPtr("shutdown"),
	}); err != nil {
		t.Fatal(err)
	}
	addr := &store.AddressEntity{
		ID: uuid.New(), MessageID: id, ChatID: chatID,
		RegionKladr: store.StrPtr("999-99.999-00.000-00.000-00.000"),
	}
	if err := s.SaveAddress(ctx, addr); err != nil {
		t.Fatal(err)
	}
	// No subscriptions saved at all — nothing can match this prefix.

	if err := a.Notify(ctx, id, chatID); err != nil {
		t.Fatalf("Notify must not error when no subscription matches: %v", err)
	}
}

// TestNotify_NilContext_ReturnsError — Finding 2 regression: when a
// subscription DOES match, a nil message.Context (real nilable hstore
// column) must fail the activity loudly (matching Java's
// message.getContext().get("supplier") NPE → activity failure/retry)
// instead of silently sending a notification with an empty supplier.
func TestNotify_NilContext_ReturnsError(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	a := &Activities{Store: s}

	const id, chatID = int64(2), int64(-200)
	msg := domain.TelegramMessage{
		ID: id, ChatID: chatID, Text: "hello",
		Date: domain.LocalDateTime{Time: time.Now()},
		From: &domain.User{ID: 1, Name: "n"},
		// Context intentionally left nil.
	}
	if err := s.SaveMessage(ctx, store.TelegramMessageEntityFrom(msg, uuid.New())); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := s.SaveTranscribe(ctx, &store.TranscribeEntity{
		ID: id, ChatID: chatID, Event: store.StrPtr("shutdown"), EventStart: &start,
	}); err != nil {
		t.Fatal(err)
	}
	regionKladr := "123-01.001-00.000-00.000-00.000"
	addr := &store.AddressEntity{
		ID: uuid.New(), MessageID: id, ChatID: chatID,
		RegionKladr: store.StrPtr(regionKladr),
	}
	if err := s.SaveAddress(ctx, addr); err != nil {
		t.Fatal(err)
	}
	// Subscription whose kladr starts with the address's region prefix, so
	// FindSubscriptionsByKladrPrefix must return it and the notify loop must
	// actually be reached for this address.
	if err := s.SaveSubscription(ctx, &store.Subscription{
		ID: uuid.New(), CreatedAt: time.Now(),
		SubscribeToKladr:    "123-01.001-02.002-00.000-04.004",
		TgID:                "555",
		SubscribeToFulltext: "г. Тирасполь, ул. Ленина",
	}); err != nil {
		t.Fatal(err)
	}

	err := a.Notify(ctx, id, chatID)
	if err == nil {
		t.Fatal("expected error for nil message.Context, got nil")
	}
	if !strings.Contains(err.Error(), "no context") {
		t.Fatalf("expected 'no context' error, got: %v", err)
	}
}
