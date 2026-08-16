package config

import (
	"testing"
	"time"
)

func TestPostgresURL_JDBC(t *testing.T) {
	got, err := PostgresURL("jdbc:postgresql://db.host:5432/events", "u", "p")
	if err != nil {
		t.Fatal(err)
	}
	want := "postgresql://u:p@db.host:5432/events"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPostgresURL_JDBCWithParams(t *testing.T) {
	got, err := PostgresURL("jdbc:postgresql://db:5432/events?sslmode=disable", "u", "p")
	if err != nil {
		t.Fatal(err)
	}
	want := "postgresql://u:p@db:5432/events?sslmode=disable"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestPostgresURL_AlreadyPostgres(t *testing.T) {
	got, err := PostgresURL("postgres://a:b@h:5432/db", "u", "p")
	if err != nil {
		t.Fatal(err)
	}
	if got != "postgres://a:b@h:5432/db" { // существующий userinfo не перетирается
		t.Fatalf("got %q", got)
	}
}

func TestParseISODuration(t *testing.T) {
	cases := map[string]time.Duration{
		"PT24H":   24 * time.Hour,
		"PT1H30M": 90 * time.Minute,
		"P1D":     24 * time.Hour,
		"P1DT12H": 36 * time.Hour,
		"PT30S":   30 * time.Second,
		"24h":     24 * time.Hour, // Go-формат тоже принимаем
	}
	for in, want := range cases {
		got, err := ParseISODuration(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got != want {
			t.Fatalf("%s: got %v want %v", in, got, want)
		}
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg := Load()
	if cfg.DBURL != "jdbc:postgresql://localhost:5432/events" ||
		cfg.DBUsername != "postgres" || cfg.DBPassword != "postgres" ||
		cfg.TemporalURL != "localhost:7233" ||
		cfg.OpenAIAPIKey != "placeholder" ||
		cfg.OpenAIBaseURL != "https://api.openai.com" ||
		cfg.GeoBaseURL != "http://localhost:8081" ||
		cfg.NATSStream != "UTILITY" ||
		cfg.NATSSubjectPrefix != "pmr.utility.event" ||
		cfg.NATSStreamMaxAge != 24*time.Hour ||
		cfg.SQSQueueName != "events-notifications" {
		t.Fatalf("defaults mismatch: %+v", cfg)
	}
}
