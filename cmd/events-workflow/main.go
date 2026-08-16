package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"events-workflow/internal/ai"
	"events-workflow/internal/config"
	"events-workflow/internal/events"
	"events-workflow/internal/geo"
	"events-workflow/internal/messaging"
	"events-workflow/internal/server"
	"events-workflow/internal/store"
	"events-workflow/internal/workflows"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	pgURL, err := config.PostgresURL(cfg.DBURL, cfg.DBUsername, cfg.DBPassword)
	if err != nil {
		return err
	}
	if err := store.Migrate(pgURL); err != nil {
		return err
	}
	pool, err := store.NewPool(ctx, pgURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	sender, err := messaging.NewSQSSender(ctx, cfg.SQSQueueName)
	if err != nil {
		return err
	}
	publisher := events.NewPublisher(events.PublisherConfig{
		URL: cfg.NATSURL, User: cfg.NATSUser, Pass: cfg.NATSPass,
		CredsFile: cfg.NATSCreds, Stream: cfg.NATSStream,
		SubjectPrefix: cfg.NATSSubjectPrefix, StreamMaxAge: cfg.NATSStreamMaxAge,
	})
	defer publisher.Close()

	aiClient := ai.NewClient(cfg.OpenAIBaseURL, cfg.OpenAIAPIKey)
	activities := &workflows.Activities{
		Store:     store.New(pool),
		Parser:    ai.NewMessageParser(aiClient),
		Adapter:   geo.NewAdapter(geo.NewClient(cfg.GeoBaseURL), ai.NewAddressPicker(aiClient)),
		Sender:    sender,
		Publisher: publisher,
	}

	temporalClient, err := client.Dial(client.Options{HostPort: cfg.TemporalURL})
	if err != nil {
		return err
	}
	defer temporalClient.Close()

	w := worker.New(temporalClient, workflows.TaskQueue, workflows.WorkerOptions())
	workflows.Register(w, activities)

	httpServer := server.New(":8080", pool.Ping)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server", "error", err)
		}
	}()

	slog.Info("events-workflow started",
		"taskQueue", workflows.TaskQueue, "temporal", cfg.TemporalURL)
	err = w.Run(worker.InterruptCh()) // блокируется до SIGINT/SIGTERM
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	return err
}
