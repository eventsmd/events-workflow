// Package config загружает конфигурацию из env-переменных.
// Имена и дефолты идентичны Java-версии (application.yaml) — деплой
// меняет только хэш образа.
package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

type Config struct {
	DBURL             string
	DBUsername        string
	DBPassword        string
	TemporalURL       string
	OpenAIAPIKey      string
	OpenAIBaseURL     string
	GeoBaseURL        string
	NATSURL           string
	NATSUser          string
	NATSPass          string
	NATSCreds         string
	NATSStream        string
	NATSSubjectPrefix string
	NATSStreamMaxAge  time.Duration
	SQSQueueName      string
}

func Load() Config {
	return Config{
		DBURL:             env("DB_URL", "jdbc:postgresql://localhost:5432/events"),
		DBUsername:        env("DB_USERNAME", "postgres"),
		DBPassword:        env("DB_PASSWORD", "postgres"),
		TemporalURL:       env("TEMPORAL_URL", "localhost:7233"),
		OpenAIAPIKey:      env("OPENAI_API_KEY", "placeholder"),
		OpenAIBaseURL:     env("OPENAI_BASE_URL", "https://api.openai.com"),
		GeoBaseURL:        env("GEO_BASE_URL", "http://localhost:8081"),
		NATSURL:           env("NATS_URL", ""),
		NATSUser:          env("NATS_USER", ""),
		NATSPass:          env("NATS_PASS", ""),
		NATSCreds:         env("NATS_CREDS", ""),
		NATSStream:        env("NATS_STREAM", "UTILITY"),
		NATSSubjectPrefix: env("NATS_SUBJECT_PREFIX", "pmr.utility.event"),
		NATSStreamMaxAge:  mustDuration(env("NATS_STREAM_MAX_AGE", "PT24H")),
		SQSQueueName:      env("AWS_SQS_QUEUE_NAME", "events-notifications"),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustDuration(s string) time.Duration {
	d, err := ParseISODuration(s)
	if err != nil {
		panic(fmt.Sprintf("invalid duration %q: %v", s, err))
	}
	return d
}

// PostgresURL превращает jdbc:postgresql://… (формат Java-версии) в
// postgresql://user:pass@…, пригодный для pgx и golang-migrate.
// Уже указанный в URL userinfo имеет приоритет над user/pass.
func PostgresURL(dbURL, user, pass string) (string, error) {
	s := strings.TrimPrefix(dbURL, "jdbc:")
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("parse DB_URL: %w", err)
	}
	if u.User == nil {
		u.User = url.UserPassword(user, pass)
	}
	return u.String(), nil
}

var isoDuration = regexp.MustCompile(
	`^P(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

// ParseISODuration понимает ISO-8601 (PT24H — формат java.time.Duration
// из старого конфига) и Go-формат (24h).
func ParseISODuration(s string) (time.Duration, error) {
	m := isoDuration.FindStringSubmatch(s)
	if m == nil {
		return time.ParseDuration(s)
	}
	var d time.Duration
	units := []time.Duration{24 * time.Hour, time.Hour, time.Minute, time.Second}
	any := false
	for i, unit := range units {
		if m[i+1] != "" {
			var n int64
			fmt.Sscanf(m[i+1], "%d", &n)
			d += time.Duration(n) * unit
			any = true
		}
	}
	if !any {
		return 0, fmt.Errorf("empty ISO-8601 duration: %q", s)
	}
	return d, nil
}
