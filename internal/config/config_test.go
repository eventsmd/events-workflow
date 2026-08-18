package config

import (
	"strings"
	"testing"
	"time"
)

func TestPostgresURL_JDBC(t *testing.T) {
	got, err := PostgresURL("jdbc:postgresql://db.host:5432/events", "u", "p")
	if err != nil {
		t.Fatal(err)
	}
	want := "postgresql://u:p@db.host:5432/events?sslmode=prefer"
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
	// existing userinfo must not be overwritten; sslmode still defaults to prefer
	if got != "postgres://a:b@h:5432/db?sslmode=prefer" {
		t.Fatalf("got %q", got)
	}
}

// TestPostgresURL_SSLMode covers the sslmode=prefer default added so
// lib/pq (used by golang-migrate) matches pgjdbc's (and pgx's) default:
// a DB_URL with no sslmode parameter — the exact form the Java service
// uses — must not fail with "SSL is not enabled on the server".
func TestPostgresURL_SSLMode(t *testing.T) {
	cases := []struct {
		name  string
		dbURL string
		want  string
	}{
		{
			name:  "no sslmode gains prefer",
			dbURL: "jdbc:postgresql://db:5432/events",
			want:  "postgresql://u:p@db:5432/events?sslmode=prefer",
		},
		{
			name:  "explicit sslmode is preserved, not duplicated",
			dbURL: "jdbc:postgresql://db:5432/events?sslmode=require",
			want:  "postgresql://u:p@db:5432/events?sslmode=require",
		},
		{
			name:  "other params are kept and sslmode=prefer is added",
			dbURL: "jdbc:postgresql://db:5432/events?application_name=x",
			want:  "postgresql://u:p@db:5432/events?application_name=x&sslmode=prefer",
		},
		{
			name:  "non-jdbc URL with explicit sslmode is untouched",
			dbURL: "postgres://a:b@h:5432/db?sslmode=disable",
			want:  "postgres://a:b@h:5432/db?sslmode=disable",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PostgresURL(tc.dbURL, "u", "p")
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			if strings.Count(got, "sslmode=") > 1 {
				t.Fatalf("sslmode duplicated: %q", got)
			}
		})
	}
}

func TestParseISODuration(t *testing.T) {
	cases := map[string]time.Duration{
		"PT24H":   24 * time.Hour,
		"PT1H30M": 90 * time.Minute,
		"P1D":     24 * time.Hour,
		"P1DT12H": 36 * time.Hour,
		"PT30S":   30 * time.Second,
		"24h":     24 * time.Hour, // Go format is also accepted
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
