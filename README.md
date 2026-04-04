# Events Workflow

Spring Boot application that processes Telegram messages about utility supply events (water/electricity shutdowns and resumptions) and notifies subscribed users.

## Architecture

```
Telegram -> [SQS/Temporal] -> EventsWorkflow -> [OpenAI] -> [KLADR API] -> [SQS] -> Notification
```

### Components

| Package | Responsibility |
|---------|---------------|
| `workflows` | Temporal workflow and activity definitions for message processing pipeline |
| `ai` | OpenAI integration for parsing messages and selecting addresses |
| `geo` | KLADR geocoding API integration for address normalization |
| `persistence` | JPA entities and Spring Data repositories (PostgreSQL) |
| `messaging` | AWS SQS message sending for subscriber notifications |
| `domain` | Domain records (value objects) |

### Processing Pipeline

1. **Save raw message** — persist the incoming Telegram message, link replies to the same incident
2. **Parse message** — use OpenAI (GPT-5-mini) to extract structured data: organization, event type, timestamps, addresses
3. **Save parsed data** — persist transcription and extracted addresses
4. **Normalize addresses** — resolve addresses via KLADR API; when multiple matches found, use AI to pick the best one
5. **Notify subscribers** — find users subscribed to affected streets and send notifications via SQS

## Tech Stack

- **Java 21**, Spring Boot 3.5
- **Temporal** — workflow orchestration
- **PostgreSQL** — data storage (with hstore extension)
- **Flyway** — database migrations
- **Spring AI + OpenAI** — message parsing and address disambiguation
- **AWS SQS** — notification delivery
- **Micrometer + Prometheus** — metrics
- **Spring Boot Actuator** — health checks

## Configuration

All configuration is via environment variables:

| Variable | Description |
|----------|-------------|
| `DB_URL` | PostgreSQL JDBC URL |
| `DB_USERNAME` | Database username |
| `DB_PASSWORD` | Database password |
| `OPENAI_API_KEY` | OpenAI API key |
| `TEMPORAL_URL` | Temporal server address |
| `GEO_BASE_URL` | KLADR geocoding API base URL |
| `AWS_ACCESS_KEY_ID` | AWS access key |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key |
| `AWS_REGION` | AWS region |
| `AWS_SQS_QUEUE_NAME` | SQS queue name for notifications |

## Build & Run

```bash
# Build
./mvnw clean package

# Run tests
./mvnw test

# Build OCI image
./mvnw spring-boot:build-image -Dspring-boot.build-image.imageName=events-workflow:latest

# Run
java -jar target/events-0.0.1.jar
```

## Database

PostgreSQL with hstore extension. Flyway migrations in `src/main/resources/db/migration/`.

### Tables

- `telegram_messages` — raw incoming Telegram messages (composite PK: id + chat_id)
- `telegram_message_transcribes` — AI-parsed message content
- `incident_address` — extracted and normalized addresses
- `subscriptions` — user subscriptions to KLADR street codes

## CI/CD

GitHub Actions workflow (`.github/workflows/build-image.yml`) builds an OCI image with Spring Boot buildpacks and pushes to GHCR on main branch pushes.
