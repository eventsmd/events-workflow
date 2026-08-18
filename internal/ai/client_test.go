package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestRetryBackoff_FitsInsideActivityBudget — the in-client retry backoff
// (2s+10s+50s = 62s for the default 4 attempts / x5 multiplier) runs INSIDE
// one Temporal activity attempt, so it must stay small relative to the
// 5-minute StartToCloseTimeout (internal/workflows/workflow.go). The
// attempts themselves are no longer part of this arithmetic: each one is
// bounded by the activity context deadline (see attemptCtx).
func TestRetryBackoff_FitsInsideActivityBudget(t *testing.T) {
	c := NewClient("http://example.invalid", "key")

	const activityBudget = 5 * time.Minute
	backoff := c.retryInitialWait
	var totalBackoff time.Duration
	for i := 1; i < c.retryMaxAttempts; i++ {
		totalBackoff += backoff
		backoff = time.Duration(float64(backoff) * c.retryMultiplier)
	}
	if totalBackoff > activityBudget/4 {
		t.Fatalf("retry backoff %v is too large a share of the %v activity budget",
			totalBackoff, activityBudget)
	}
}

// TestNewClient_HasNoFixedRequestTimeout — regression test for the
// production failure "Post .../chat/completions: context deadline exceeded
// (Client.Timeout exceeded while awaiting headers)". A fixed
// http.Client.Timeout caps every call regardless of how much activity
// budget is left; Java (Spring RestClient, no read timeout configured)
// had no such cap and let a slow gpt-5-mini call run until Temporal's
// StartToCloseTimeout. The deadline must come from the context instead.
func TestNewClient_HasNoFixedRequestTimeout(t *testing.T) {
	if to := NewClient("http://example.invalid", "key").http.Timeout; to != 0 {
		t.Fatalf("http.Client.Timeout = %v, want 0 so the context deadline governs", to)
	}
}

// TestClient_Chat_ContextDeadlineOutranksSafetyNet — when the caller (a
// Temporal activity) supplies a deadline, that deadline alone bounds the
// attempt. The safety net must not shorten a call the activity budget
// still has room for.
func TestClient_Chat_ContextDeadlineOutranksSafetyNet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond) // longer than the safety net below
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-key")
	c.maxRequestTimeout = 50 * time.Millisecond // would abort the call if it applied

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	got, err := c.Chat(ctx, "hi")
	if err != nil {
		t.Fatalf("call aborted despite ample context budget: %v", err)
	}
	if got != "ok" {
		t.Fatalf("got %q", got)
	}
}

// TestClient_Chat_SafetyNetBoundsDeadlinelessContext — with no deadline on
// the context there is no activity budget to inherit, so maxRequestTimeout
// must still stop a stuck request from hanging forever.
func TestClient_Chat_SafetyNetBoundsDeadlinelessContext(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // never responds within the test's lifetime
	}))
	defer srv.Close()
	// Deferred LIFO: unblock the handler first, otherwise srv.Close waits on it forever.
	defer close(block)

	c := NewClient(srv.URL, "test-key")
	c.retryMaxAttempts = 1                      // isolate a single attempt's timeout behavior
	c.maxRequestTimeout = 50 * time.Millisecond // fast for the test

	start := time.Now()
	_, err := c.Chat(context.Background(), "hi") // no deadline

	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Chat took %v, want it bounded by the client's request timeout", elapsed)
	}
}

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
