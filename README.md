# Events Workflow

Go service that processes Telegram messages about utility supply events (water/electricity shutdowns and resumptions) and notifies subscribed users.

## Architecture

```
Telegram -> [SQS/Temporal] -> EventsWorkflow -> [OpenAI] -> [KLADR API] -> [SQS] -> Notification
```

### Components

| Package | Responsibility |
|---------|---------------|
| `internal/workflows` | Temporal workflow and activity definitions for the message processing pipeline |
| `internal/ai` | OpenAI integration for parsing messages and selecting addresses |
| `internal/geo` | KLADR geocoding API integration for address normalization |
| `internal/store` | pgx/v5 repositories (PostgreSQL): messages, transcriptions, addresses, subscriptions |
| `internal/messaging` | AWS SQS message sending for subscriber notifications |
| `internal/events` | NATS JetStream publisher for the public utility-event feed |
| `internal/domain` | Domain value types (messages, addresses, transcriptions, `LocalDateTime`) |
| `internal/kladr` | `KladrCode` parsing/level/prefix logic |
| `internal/server` | HTTP server: health checks and Prometheus metrics |
| `internal/config` | Environment-variable configuration loading |

### Processing Pipeline

1. **Save raw message** — persist the incoming Telegram message, link replies to the same incident
2. **Parse message** — use OpenAI (`gpt-5-mini`) to extract structured data: organization, event type, timestamps, addresses
3. **Save parsed data** — persist transcription and extracted addresses
4. **Normalize addresses** — resolve addresses via the KLADR API; when multiple matches are found, use AI to pick the best one
5. **Notify subscribers** — find users subscribed to affected streets (by KLADR prefix) and send notifications via SQS
6. **Publish event** — publish the parsed event to the public NATS hub (`pmr.utility.event.<supplier>.<event>`), best-effort and non-blocking, in parallel with notify

Activity retries: StartToCloseTimeout 5 min, initial interval 1s, max interval 300s, backoff coefficient 5.0, up to 3 attempts.

## Tech Stack

- **Go** (see `go.mod` for the exact version)
- **Temporal** (`go.temporal.io/sdk`) — workflow orchestration
- **PostgreSQL** (`jackc/pgx/v5`) — data storage (with hstore extension)
- **golang-migrate** — database migrations, embedded in the binary
- **OpenAI** (`openai/openai-go`) — message parsing and address disambiguation
- **AWS SQS** (`aws/aws-sdk-go-v2`) — notification delivery
- **NATS JetStream** (`nats-io/nats.go`) — public event feed
- **Prometheus** (`prometheus/client_golang`) — metrics
- **net/http** — health checks (no framework)

No ORM, no dependency-injection framework; wiring happens in `cmd/events-workflow/main.go`.

## Configuration

All configuration is via environment variables. Names and defaults are unchanged from the previous (Java) implementation, so deploys only need the image hash to change.

| Variable | Default | Notes |
|----------|---------|-------|
| `DB_URL` | `jdbc:postgresql://localhost:5432/events` | Both `jdbc:postgresql://…` (prefix stripped) and `postgres://…` are accepted |
| `DB_USERNAME` | `postgres` | |
| `DB_PASSWORD` | `postgres` | |
| `TEMPORAL_URL` | `localhost:7233` | |
| `OPENAI_API_KEY` | `placeholder` | |
| `GEO_BASE_URL` | `http://localhost:8081` | |
| `NATS_URL` | empty | empty ⇒ publish is skipped |
| `NATS_USER` / `NATS_PASS` | empty | |
| `NATS_CREDS` | empty | path to a NATS creds file; takes priority over user/pass |
| `NATS_STREAM` | `UTILITY` | |
| `NATS_SUBJECT_PREFIX` | `pmr.utility.event` | |
| `NATS_STREAM_MAX_AGE` | `PT24H` | ISO-8601 duration; the Go-style `24h` format is also accepted |
| `AWS_ACCESS_KEY_ID` | `test` | aws-sdk-go-v2 also reads the standard AWS env vars directly |
| `AWS_SECRET_ACCESS_KEY` | `test` | |
| `AWS_REGION` | `us-east-1` | |
| `AWS_SQS_QUEUE_NAME` | `events-notifications` | |

Port `8080` is fixed.

## Build & Run

```bash
# Build
go build ./cmd/events-workflow

# Run unit tests (fast, no external services)
go test -short ./...

# Run the full test suite, including integration tests
# (spins up Postgres/LocalStack/NATS via testcontainers-go — requires Docker)
go test ./...

# Run
./events-workflow
```

## Database

PostgreSQL with the hstore extension. Migrations live in `migrations/` and are embedded in the binary, applied automatically at startup via `golang-migrate`.

### Tables

- `telegram_messages` — raw incoming Telegram messages (composite PK: id + chat_id)
- `telegram_message_transcribes` — AI-parsed message content
- `incident_address` — extracted and normalized addresses
- `subscriptions` — user subscriptions to KLADR street codes

## HTTP Endpoints

- `/actuator/health`, `/actuator/health/liveness`, `/actuator/health/readiness` — health checks (paths kept unchanged so existing k8s manifests keep working)
- `/actuator/prometheus` — Prometheus metrics (standard `client_golang` process/Go runtime metrics; JVM-specific metric names no longer apply)

## Docker

```bash
docker build -t events-workflow:local .
```

Multi-stage build: `golang` for compiling a static (`CGO_ENABLED=0`) binary, `gcr.io/distroless/static-debian12:nonroot` as the runtime base. Exposes port `8080`.

## CI/CD

GitHub Actions workflow (`.github/workflows/build-image.yml`) runs `go test -short ./...`, builds the OCI image, and pushes it to GHCR (`ghcr.io/<repo>:<sha>`) on pushes to `main`. Integration tests (Docker-based, via testcontainers-go) are not run in CI; run `go test ./...` locally before releasing if you want that coverage.
