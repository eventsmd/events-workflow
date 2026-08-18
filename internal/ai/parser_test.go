package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"events-workflow/internal/domain"
)

// fakeOpenAI starts an httptest server responding with fixed content;
// records the last sent prompt.
func fakeOpenAI(t *testing.T, content string, lastPrompt *string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth %q", got)
		}
		var req struct {
			Model       string  `json:"model"`
			Temperature float64 `json:"temperature"`
			Messages    []struct {
				Role, Content string
			} `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "gpt-5-mini" || req.Temperature != 1.0 {
			t.Errorf("model=%s temp=%v", req.Model, req.Temperature)
		}
		if lastPrompt != nil && len(req.Messages) == 1 {
			*lastPrompt = req.Messages[0].Content
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{
				"role": "assistant", "content": content,
			}}},
		})
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "test-key")
}

func TestParse_ValidJSON_AssignsAddressIDs(t *testing.T) {
	c := fakeOpenAI(t, "```json\n{\"organization\":\"Водоканал\",\"short_description\":\"d\",\"event\":\"shutdown\",\"event_start\":\"2025-12-02T09:00\",\"addresses\":[{\"city\":\"Тирасполь\",\"street\":\"Ленина\",\"street_type\":\"ул.\"}]}\n```", nil)
	p := NewMessageParser(c)
	got, err := p.Parse(context.Background(),
		domain.LocalDateTime{Time: time.Date(2025, 12, 1, 22, 50, 0, 0, time.UTC)}, "msg")
	if err != nil {
		t.Fatal(err)
	}
	if got.Event != "shutdown" || got.Organization != "Водоканал" {
		t.Fatalf("%+v", got)
	}
	if got.Addresses[0].ID == uuid.Nil {
		t.Fatal("address must get a generated UUID")
	}
}

// TestParse_TrailingContentAfterJSON_Ignored — Java's Jackson readValue does
// not enable FAIL_ON_TRAILING_TOKENS, so text the model appends after the
// JSON object (e.g. "Note: times are approximate.") is silently ignored.
// Go's json.Unmarshal would instead fail with "invalid character ... after
// top-level value", which previously burned all 3 Temporal activity attempts
// on an otherwise-parseable response.
func TestParse_TrailingContentAfterJSON_Ignored(t *testing.T) {
	c := fakeOpenAI(t, "```json\n{\"organization\":\"Водоканал\",\"short_description\":\"d\",\"event\":\"shutdown\",\"event_start\":\"2025-12-02T09:00\",\"addresses\":[{\"city\":\"Тирасполь\",\"street\":\"Ленина\",\"street_type\":\"ул.\"}]}\n```\nNote: times are approximate.", nil)
	p := NewMessageParser(c)
	got, err := p.Parse(context.Background(),
		domain.LocalDateTime{Time: time.Date(2025, 12, 1, 22, 50, 0, 0, time.UTC)}, "msg")
	if err != nil {
		t.Fatalf("expected trailing content to be ignored, got error: %v", err)
	}
	if got.Event != "shutdown" || got.Organization != "Водоканал" {
		t.Fatalf("%+v", got)
	}
}

func TestParse_NullResponse(t *testing.T) {
	p := NewMessageParser(fakeOpenAI(t, "null", nil))
	got, err := p.Parse(context.Background(), domain.LocalDateTime{}, "мусор")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("want nil, got %+v", got)
	}
}

func TestParse_PromptContainsMessageAndTime(t *testing.T) {
	var prompt string
	p := NewMessageParser(fakeOpenAI(t, "null", &prompt))
	tm := domain.LocalDateTime{Time: time.Date(2025, 12, 1, 22, 50, 0, 0, time.UTC)}
	p.Parse(context.Background(), tm, "Отключение воды")
	// Java: prompt + "\nMessage at {LocalDateTime.toString()}:\n{message}"
	want := "\nMessage at 2025-12-01T22:50:\nОтключение воды"
	if len(prompt) == 0 || prompt[len(prompt)-len(want):] != want {
		t.Fatalf("prompt suffix mismatch:\n%s", prompt)
	}
}
