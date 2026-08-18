// Package integration — end-to-end integration test (port of
// EventsWorkflowIntegrationTest.java): brings up postgres + temporal
// auto-setup + localstack SQS in docker, fake geo/OpenAI HTTP services,
// starts worker in-process, and runs workflow BY NAME (as the external
// Telegram service does) with JSON-like payload — this is the wire-compatibility
// check, which matters most.
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tclocalstack "github.com/testcontainers/testcontainers-go/modules/localstack"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"events-workflow/internal/ai"
	"events-workflow/internal/events"
	"events-workflow/internal/geo"
	"events-workflow/internal/messaging"
	"events-workflow/internal/store"
	"events-workflow/internal/workflows"
)

const queueName = "test-notifications"

func TestFullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// --- infrastructure
	net, err := network.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { net.Remove(context.Background()) })

	pg, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("events"),
		tcpostgres.WithUsername("test"), tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
		network.WithNetwork([]string{"postgres"}, net),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testcontainers.TerminateContainer(pg) })

	temporalC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "temporalio/auto-setup:latest",
			ExposedPorts: []string{"7233/tcp"},
			Env: map[string]string{
				"DB": "postgres12", "DB_PORT": "5432",
				"POSTGRES_USER": "test", "POSTGRES_PWD": "test",
				"POSTGRES_SEEDS": "postgres",
			},
			Networks:   []string{net.Name},
			WaitingFor: wait.ForListeningPort("7233/tcp").WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testcontainers.TerminateContainer(temporalC) })

	ls, err := tclocalstack.Run(ctx, "localstack/localstack:3")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testcontainers.TerminateContainer(ls) })
	lsPort, err := ls.MappedPort(ctx, "4566/tcp")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ENDPOINT_URL", "http://localhost:"+lsPort.Port())

	// --- fake geo and OpenAI
	geoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"full_address":"г. Тирасполь, ул. Ленина",
			"region":{"kladr":"123-01.001-00.000-00.000-00.000","name":"Приднестровье","type":"р."},
			"city":{"kladr":"123-01.001-02.002-00.000-00.000","name":"Тирасполь","type":"г."},
			"street":{"kladr":"123-01.001-02.002-00.000-04.004","name":"Ленина","type":"ул."}}]`))
	}))
	t.Cleanup(geoSrv.Close)

	transcription := `{"organization":"Водоканал","short_description":"Плановые работы",` +
		`"event":"shutdown","event_start":"2025-12-02T09:00","event_stop":"2025-12-02T18:00",` +
		`"addresses":[{"city":"Тирасполь","street":"Ленина","street_type":"ул.",` +
		`"house":{"numbers":["1","2"]}}]}`
	aiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{
			"message": map[string]any{"role": "assistant", "content": transcription}}}})
	}))
	t.Cleanup(aiSrv.Close)

	// --- migrations, store, subscription
	pgURL, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(pgURL); err != nil {
		t.Fatal(err)
	}
	pool, err := store.NewPool(ctx, pgURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := store.New(pool)
	if err := st.SaveSubscription(ctx, &store.Subscription{
		ID: uuid.New(), CreatedAt: time.Now(),
		SubscribeToKladr: "123-01.001-02.002-00.000-04.004", TgID: "777",
		SubscribeToFulltext: "г. Тирасполь, ул. Ленина",
	}); err != nil {
		t.Fatal(err)
	}

	// --- SQS queue
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sqsClient := awssqs.NewFromConfig(awsCfg)
	created, err := sqsClient.CreateQueue(ctx, &awssqs.CreateQueueInput{
		QueueName: aws.String(queueName)})
	if err != nil {
		t.Fatal(err)
	}
	sender, err := messaging.NewSQSSender(ctx, queueName)
	if err != nil {
		t.Fatal(err)
	}

	// --- worker
	aiClient := ai.NewClient(aiSrv.URL, "test")
	activities := &workflows.Activities{
		Store:     st,
		Parser:    ai.NewMessageParser(aiClient),
		Adapter:   geo.NewAdapter(geo.NewClient(geoSrv.URL), ai.NewAddressPicker(aiClient)),
		Sender:    sender,
		Publisher: events.NewPublisher(events.PublisherConfig{}), // NATS disabled — skip, like with empty NATS_URL
	}
	temporalEndpoint, err := temporalC.PortEndpoint(ctx, "7233/tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	tc, err := client.Dial(client.Options{HostPort: temporalEndpoint})
	if err != nil {
		t.Fatal(err)
	}
	defer tc.Close()
	w := worker.New(tc, workflows.TaskQueue, worker.Options{})
	workflows.Register(w, activities)
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer w.Stop()

	// --- start workflow BY NAME (as the external service does)
	run, err := tc.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID: "it-1", TaskQueue: workflows.TaskQueue,
	}, workflows.WorkflowName, map[string]any{
		"id": 100, "chat_id": -5,
		"from":    map[string]any{"id": 1, "name": "Водоканал"},
		"text":    "Отключение воды по ул. Ленина 1, 2",
		"date":    "2025-12-01T22:50",
		"context": map[string]string{"supplier": "water"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := run.Get(ctx, nil); err != nil {
		t.Fatal(err)
	}

	// --- DB checks
	msg, err := st.FindMessage(ctx, 100, -5)
	if err != nil || msg == nil || msg.IncidentID == nil {
		t.Fatalf("message not saved: %+v %v", msg, err)
	}
	tr, err := st.FindTranscribe(ctx, 100, -5)
	if err != nil || tr == nil || tr.Event == nil || *tr.Event != "shutdown" {
		t.Fatalf("transcribe: %+v %v", tr, err)
	}
	addrs, err := st.FindAddressesByMessage(ctx, 100, -5)
	if err != nil || len(addrs) != 1 {
		t.Fatalf("addresses: %v %v", addrs, err)
	}
	if addrs[0].StreetKladr == nil || *addrs[0].StreetKladr != "123-01.001-02.002-00.000-04.004" {
		t.Fatalf("address not enriched: %+v", addrs[0])
	}

	// --- SQS notification check
	recv, err := sqsClient.ReceiveMessage(ctx, &awssqs.ReceiveMessageInput{
		QueueUrl: created.QueueUrl, MaxNumberOfMessages: 1, WaitTimeSeconds: 10,
	})
	if err != nil || len(recv.Messages) != 1 {
		t.Fatalf("sqs: %v %v", recv, err)
	}
	var notif map[string]any
	json.Unmarshal([]byte(*recv.Messages[0].Body), &notif)
	if notif["telegram_id"] != float64(777) || notif["topic"] != "water" {
		t.Fatalf("%v", notif)
	}
	// Assert the FULL notification text, not just a substring — this is what
	// actually catches wire/format drift in NotificationText/FormatHouses
	// (emoji, service name, address + house numbers, date format, description).
	const wantMessage = "💧 Отключение услуги водоснабжения по адресу " +
		"«г. Тирасполь, ул. Ленина, д. 1, 2» с 02-12-2025 09:00\n\nПлановые работы"
	if notif["message"] != wantMessage {
		t.Fatalf("notification text mismatch:\n got: %q\nwant: %q", notif["message"], wantMessage)
	}
}
