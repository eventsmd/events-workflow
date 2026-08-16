package store

import (
	"context"
	"testing"
)

// TestNewPool_ConfiguresPoolLimits — parity with Java's HikariCP defaults
// (maximumPoolSize=10, connectionTimeout=30s, neither overridden in
// application.yaml). pgxpool.NewWithConfig doesn't dial eagerly (MinConns
// defaults to 0), so this runs without a live database.
func TestNewPool_ConfiguresPoolLimits(t *testing.T) {
	pool, err := NewPool(context.Background(), "postgres://user:pass@localhost:1/db")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	cfg := pool.Config()
	if cfg.MaxConns != 10 {
		t.Fatalf("MaxConns = %d, want 10", cfg.MaxConns)
	}
	if cfg.ConnConfig.ConnectTimeout != acquireTimeout {
		t.Fatalf("ConnectTimeout = %v, want %v", cfg.ConnConfig.ConnectTimeout, acquireTimeout)
	}
}
