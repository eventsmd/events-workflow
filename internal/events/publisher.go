package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type PublisherConfig struct {
	URL           string
	User          string
	Pass          string
	CredsFile     string
	Stream        string
	SubjectPrefix string
	StreamMaxAge  time.Duration
}

// Publisher — port of NatsEventPublisher: lazy connection, stream creation/update,
// publish with dedup by Nats-Msg-Id = incidentId. All errors are logged
// and swallowed — publishing is best-effort and must not drop message processing.
type Publisher struct {
	cfg PublisherConfig

	mu   sync.Mutex
	conn *nats.Conn
	js   jetstream.JetStream
}

func NewPublisher(cfg PublisherConfig) *Publisher { return &Publisher{cfg: cfg} }

func (p *Publisher) Publish(ctx context.Context, ev UtilityEvent) {
	if p.cfg.URL == "" {
		slog.Debug("NATS_URL not set — skipping event publish", "incidentId", ev.IncidentID)
		return
	}
	js, err := p.ensureConnected(ctx)
	if err != nil {
		slog.Error("Failed to connect to NATS", "incidentId", ev.IncidentID, "error", err)
		return
	}
	subject := SubjectFor(p.cfg.SubjectPrefix, ev.Supplier, ev.Event)
	data, err := json.Marshal(ev)
	if err != nil {
		slog.Error("Failed to marshal utility event", "incidentId", ev.IncidentID, "error", err)
		return
	}
	_, err = js.Publish(ctx, subject, data, jetstream.WithMsgID(ev.IncidentID))
	if err != nil {
		slog.Error("Failed to publish utility event to NATS",
			"incidentId", ev.IncidentID, "error", err)
		return
	}
	slog.Info("Published utility event", "incidentId", ev.IncidentID, "subject", subject)
}

func (p *Publisher) ensureConnected(ctx context.Context) (jetstream.JetStream, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.js != nil && p.conn != nil && p.conn.IsConnected() {
		return p.js, nil
	}
	opts := []nats.Option{nats.Timeout(5 * time.Second)}
	if p.cfg.CredsFile != "" {
		opts = append(opts, nats.UserCredentials(p.cfg.CredsFile))
	} else if p.cfg.User != "" {
		opts = append(opts, nats.UserInfo(p.cfg.User, p.cfg.Pass))
	}
	conn, err := nats.Connect(p.cfg.URL, opts...)
	if err != nil {
		return nil, err
	}
	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     p.cfg.Stream,
		Subjects: []string{p.cfg.SubjectPrefix + ".>"},
		Storage:  jetstream.FileStorage,
		MaxAge:   p.cfg.StreamMaxAge,
	})
	if err != nil {
		conn.Close()
		return nil, err
	}
	// The previous connection (if any) may still be alive and
	// auto-reconnecting in the background (nats.go retries up to 60 times
	// at 2s intervals by default) even though IsConnected() reported false
	// here — e.g. during a transient blip. Close it before overwriting so
	// repeated blips don't accumulate orphaned connections and goroutines.
	if p.conn != nil {
		p.conn.Close()
	}
	p.conn, p.js = conn, js
	return js, nil
}

func (p *Publisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		p.conn.Close()
		p.conn, p.js = nil, nil
	}
}
