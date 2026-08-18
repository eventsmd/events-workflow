package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// TestRequestTimeout_FitsInsideActivityBudget — regression test for the
// retry budget exceeding Temporal's 5-minute StartToCloseTimeout
// (internal/workflows/workflow.go). Worst case is
// retryMaxAttempts*requestTimeout (every attempt maxing out its HTTP
// timeout) plus the backoff between attempts
// (retryInitialWait * (1 + multiplier + multiplier^2) = 2s+10s+50s = 62s
// for the default 4 attempts / ×5 multiplier). That must leave real margin
// inside 300s, not just barely fit, or Temporal kills the activity
// mid-retry.
func TestRequestTimeout_FitsInsideActivityBudget(t *testing.T) {
	c := NewClient("http://example.invalid", "key")
	if c.http.Timeout != requestTimeout {
		t.Fatalf("NewClient must configure http.Client.Timeout = requestTimeout, got %v", c.http.Timeout)
	}

	const activityBudget = 5 * time.Minute
	const marginFloor = 30 * time.Second // must leave at least this much slack

	backoff := c.retryInitialWait
	var totalBackoff time.Duration
	for i := 1; i < c.retryMaxAttempts; i++ {
		totalBackoff += backoff
		backoff = time.Duration(float64(backoff) * c.retryMultiplier)
	}
	worstCase := time.Duration(c.retryMaxAttempts)*requestTimeout + totalBackoff

	if worstCase >= activityBudget {
		t.Fatalf("worst case %v does not fit inside the %v activity budget", worstCase, activityBudget)
	}
	if margin := activityBudget - worstCase; margin < marginFloor {
		t.Fatalf("worst case %v leaves only %v of margin inside %v, want at least %v",
			worstCase, margin, activityBudget, marginFloor)
	}
}

// TestClient_Chat_RespectsPerRequestTimeout — verifies that a single HTTP
// attempt is actually bounded by http.Client.Timeout (not the old 120s),
// so a stuck request can't by itself consume the whole retry budget.
func TestClient_Chat_RespectsPerRequestTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // never responds within the test's lifetime
	}))
	defer srv.Close()
	// Deferred LIFO: unblock the handler first, otherwise srv.Close waits on it forever.
	defer close(block)

	c := NewClient(srv.URL, "test-key")
	c.retryMaxAttempts = 1                 // isolate a single attempt's timeout behavior
	c.http.Timeout = 50 * time.Millisecond // fast for the test; production uses requestTimeout

	start := time.Now()
	_, err := c.Chat(context.Background(), "hi")
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
