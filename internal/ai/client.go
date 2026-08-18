// Package ai — OpenAI calls. Lightweight chat completions client on net/http
// (instead of Spring AI): model gpt-5-mini, temperature 1.0, prompts are a literal
// port of MessageParser.java / AddressPicker.java.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const model = "gpt-5-mini"

// Retry defaults — Spring AI's RetryUtils.DEFAULT_RETRY_TEMPLATE retried
// 429/5xx responses up to 10 attempts (2s initial backoff, ×5 multiplier)
// INSIDE a single Temporal activity attempt. We cap attempts lower than
// Java's 10 (4) so the backoff between them (2s+10s+50s = 62s) stays small
// relative to the 5-minute (300s) StartToCloseTimeout — Temporal's own
// 3-attempt activity retry policy is untouched and still covers anything
// that outlasts this budget.
//
// There is deliberately no fixed per-request timeout. Java configured none
// (Spring's RestClient defaults to no read timeout), so a slow gpt-5-mini
// call simply ran until Temporal killed the activity at 300s. A fixed cap
// here caused the opposite: every call died at 45s with "Client.Timeout
// exceeded while awaiting headers", and since a timeout is not a retryable
// status, the activity failed immediately. Each attempt now inherits the
// activity's own deadline instead — see attemptCtx.
const (
	defaultRetryMaxAttempts = 4
	defaultRetryInitialWait = 2 * time.Second
	defaultRetryMultiplier  = 5.0

	// defaultMaxRequestTimeout only applies when the caller's context has
	// no deadline of its own (tests, or any use outside a Temporal
	// activity). Inside an activity the context deadline is always the
	// tighter, and more meaningful, bound.
	defaultMaxRequestTimeout = 240 * time.Second
)

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client

	retryMaxAttempts  int
	retryInitialWait  time.Duration
	retryMultiplier   float64
	maxRequestTimeout time.Duration
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{baseURL: baseURL, apiKey: apiKey,
		// No http.Client.Timeout: the per-attempt deadline comes from the
		// context (attemptCtx), so a slow call can use the activity budget.
		http:              &http.Client{},
		retryMaxAttempts:  defaultRetryMaxAttempts,
		retryInitialWait:  defaultRetryInitialWait,
		retryMultiplier:   defaultRetryMultiplier,
		maxRequestTimeout: defaultMaxRequestTimeout,
	}
}

type chatRequest struct {
	Model       string        `json:"model"`
	Temperature float64       `json:"temperature"`
	Messages    []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Chat sends a single chat-completion request, retrying on 429/5xx
// responses with exponential backoff — a port of Spring AI's retry
// behavior, which otherwise a Java→Go move would silently drop, letting a
// 429 burst that Java absorbed exhaust Temporal's 3 activity attempts.
func (c *Client) Chat(ctx context.Context, prompt string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model:       model,
		Temperature: 1.0,
		Messages:    []chatMessage{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", err
	}

	wait := c.retryInitialWait
	var lastErr error
	for attempt := 1; attempt <= c.retryMaxAttempts; attempt++ {
		content, retryable, err := c.doChat(ctx, body)
		if err == nil {
			return content, nil
		}
		lastErr = err
		if !retryable || attempt == c.retryMaxAttempts {
			return "", err
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(wait):
		}
		wait = time.Duration(float64(wait) * c.retryMultiplier)
	}
	return "", lastErr
}

// attemptCtx bounds one HTTP attempt. A Temporal activity context already
// carries a deadline derived from StartToCloseTimeout, and that is the
// budget the call is genuinely allowed to use, so it is passed through
// untouched. Only a deadline-less context gets the maxRequestTimeout
// safety net, so a stuck request cannot hang forever.
func (c *Client) attemptCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.maxRequestTimeout)
}

// doChat performs one HTTP attempt. retryable reports whether the failure
// is a 429/5xx worth retrying (per Spring AI's default retry predicate);
// any other error (network failure, malformed body, 4xx other than 429) is
// returned as-is and not retried.
func (c *Client) doChat(ctx context.Context, body []byte) (content string, retryable bool, err error) {
	ctx, cancel := c.attemptCtx(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, fmt.Errorf("openai: read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		msg := ""
		var out chatResponse
		if json.Unmarshal(raw, &out) == nil && out.Error != nil {
			msg = out.Error.Message
		}
		retryable = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return "", retryable, fmt.Errorf("openai: status %d: %s", resp.StatusCode, msg)
	}

	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", false, fmt.Errorf("openai: decode: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", false, fmt.Errorf("openai: empty choices")
	}
	return out.Choices[0].Message.Content, false, nil
}
