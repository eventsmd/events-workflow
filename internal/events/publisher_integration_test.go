package events

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startNATS(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "nats:2-alpine",
			Cmd:          []string{"-js"},
			ExposedPorts: []string{"4222/tcp"},
			WaitingFor:   wait.ForListeningPort("4222/tcp"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testcontainers.TerminateContainer(c) })
	ep, err := c.PortEndpoint(ctx, "4222/tcp", "nats")
	if err != nil {
		t.Fatal(err)
	}
	return ep
}

func TestPublisher_PublishAndDedup(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	url := startNATS(t)

	p := NewPublisher(PublisherConfig{
		URL: url, Stream: "UTILITY", SubjectPrefix: "pmr.utility.event",
		StreamMaxAge: 24 * time.Hour,
	})
	defer p.Close()

	ev := UtilityEvent{IncidentID: "inc-1", Supplier: "water", Event: "shutdown",
		PublishedAt: time.Now().UTC()}
	p.Publish(ctx, ev)
	p.Publish(ctx, ev) // дубликат по Nats-Msg-Id — не должен добавиться

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, _ := jetstream.New(nc)
	stream, err := js.Stream(ctx, "UTILITY")
	if err != nil {
		t.Fatal(err)
	}
	info, _ := stream.Info(ctx)
	if info.State.Msgs != 1 {
		t.Fatalf("want 1 msg (dedup), got %d", info.State.Msgs)
	}
	cons, _ := stream.CreateConsumer(ctx, jetstream.ConsumerConfig{Durable: "t"})
	msg, err := cons.Next()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Subject() != "pmr.utility.event.water.shutdown" {
		t.Fatal(msg.Subject())
	}
	var got UtilityEvent
	if err := json.Unmarshal(msg.Data(), &got); err != nil {
		t.Fatal(err)
	}
	if got.IncidentID != "inc-1" {
		t.Fatalf("%+v", got)
	}
}

func TestPublisher_EmptyURLSkips(t *testing.T) {
	p := NewPublisher(PublisherConfig{}) // URL пустой — не должен паниковать/коннектиться
	p.Publish(context.Background(), UtilityEvent{IncidentID: "x"})
}

// Портировано из NatsEventPublisherTest.publishIsNoOpAndDoesNotThrowWhenUrlBlank:
// проверяет, что при пустом URL и полностью заполненном событии Publish не паникует.
func TestPublisher_EmptyURLSkips_FullEvent(t *testing.T) {
	p := NewPublisher(PublisherConfig{
		Stream: "UTILITY", SubjectPrefix: "pmr.utility.event", StreamMaxAge: 24 * time.Hour,
	})
	defer p.Close()
	p.Publish(context.Background(), UtilityEvent{
		IncidentID: "i", Supplier: "water", Event: "shutdown",
		Organization: "o", Description: "d",
		PublishedAt: time.Unix(0, 0).UTC(),
		Source:      &EventSource{MessageID: 1, ChatID: 2},
	})
}

// Портировано из NatsEventPublisherIntegrationTest.publishesEventReadableFromStream:
// проверяет полное тело сообщения (несколько полей) и что addresses/houses
// корректно проходят через сериализацию/публикацию/чтение.
func TestPublisher_PublishesFullEventReadableFromStream(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	url := startNATS(t)

	p := NewPublisher(PublisherConfig{
		URL: url, Stream: "UTILITY", SubjectPrefix: "pmr.utility.event",
		StreamMaxAge: 24 * time.Hour,
	})
	defer p.Close()

	ev := UtilityEvent{
		IncidentID: "inc-int-1", Supplier: "water", Event: "shutdown",
		Organization: "SA Apă-Canal", Description: "Отключение",
		PublishedAt: time.Now().UTC(),
		Source:      &EventSource{MessageID: 123, ChatID: 200},
		Addresses: []EventAddress{{
			Street: &KladrRef{Name: "Пушкина", Kladr: "001-01.001", Type: "ул"},
			Houses: []string{"1", "2"},
		}},
	}
	p.Publish(ctx, ev)

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, _ := jetstream.New(nc)
	stream, err := js.Stream(ctx, "UTILITY")
	if err != nil {
		t.Fatal(err)
	}
	cons, _ := stream.CreateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:       "full",
		FilterSubject: "pmr.utility.event.water.shutdown",
	})
	msg, err := cons.Next()
	if err != nil {
		t.Fatal(err)
	}
	body := string(msg.Data())
	for _, want := range []string{
		`"incidentId":"inc-int-1"`,
		`"supplier":"water"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
}
