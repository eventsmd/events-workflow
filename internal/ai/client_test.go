package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func fastRetryClient(baseURL string) *Client {
	c := NewClient(baseURL, "test-key")
	// Keep the test fast: same shape (max attempts, ×5 multiplier), tiny waits.
	c.retryInitialWait = time.Millisecond
	return c
}

// TestClient_Chat_RetriesOn429ThenSucceeds — Spring AI retried 429/5xx
// with backoff inside a single activity attempt; without this, a 429 burst
// Java absorbed now exhausts Temporal's 3 activity attempts.
func TestClient_Chat_RetriesOn429ThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"message":"rate limited"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := fastRetryClient(srv.URL)
	got, err := c.Chat(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ok" {
		t.Fatalf("got %q", got)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls (2 failed + 1 success), got %d", calls)
	}
}

// TestClient_Chat_RetriesOn5xxThenSucceeds mirrors the 429 case for a
// transient 503.
func TestClient_Chat_RetriesOn5xxThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := fastRetryClient(srv.URL)
	got, err := c.Chat(context.Background(), "hi")
	if err != nil {
		t.Fatal(err)
	}
	if got != "ok" {
		t.Fatalf("got %q", got)
	}
}

// TestClient_Chat_NonRetryableStatus_FailsImmediately — a plain 400 must
// not be retried (only 429/5xx are, per Spring AI's default predicate).
func TestClient_Chat_NonRetryableStatus_FailsImmediately(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer srv.Close()

	c := fastRetryClient(srv.URL)
	if _, err := c.Chat(context.Background(), "hi"); err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 call for a non-retryable status, got %d", calls)
	}
}

// TestClient_Chat_ExhaustsAttempts_ReturnsLastError — a persistent 429
// must not retry forever; it must give up after retryMaxAttempts and
// surface the last error so Temporal's own retry policy can take over.
func TestClient_Chat_ExhaustsAttempts_ReturnsLastError(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := fastRetryClient(srv.URL)
	_, err := c.Chat(context.Background(), "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != int32(c.retryMaxAttempts) {
		t.Fatalf("expected %d calls, got %d", c.retryMaxAttempts, calls)
	}
}

// TestClient_Chat_HonorsContextCancellation — a cancelled context must
// abort the retry wait instead of sleeping it out.
func TestClient_Chat_HonorsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	c.retryInitialWait = time.Hour // would hang the test if ctx weren't honored

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := c.Chat(ctx, "hi")
	if err == nil {
		t.Fatal("expected error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Chat did not honor context cancellation, took %v", elapsed)
	}
}
