// Package domain — domain types, 1:1 with Java records. JSON tags replicate
// Jackson names: @JsonProperty → snake_case, no annotation → record field name.
package domain

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type User struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type MessageReference struct {
	ID     int64 `json:"id"`
	ChatID int64 `json:"chat_id"`
}

type TelegramMessage struct {
	ID          int64             `json:"id"`
	ChatID      int64             `json:"chat_id"`
	From        *User             `json:"from"`
	Text        string            `json:"text"`
	Date        LocalDateTime     `json:"date"`
	ReplyTo     *MessageReference `json:"reply_to"`
	ServiceName string            `json:"service_name"`
	Context     map[string]string `json:"context"`
}

type House struct {
	Numbers []string   `json:"numbers"`
	Ranges  [][]string `json:"ranges"`
}

type Address struct {
	ID         uuid.UUID `json:"id"`
	City       string    `json:"city"`
	Street     string    `json:"street"`
	StreetType string    `json:"street_type"`
	House      *House    `json:"house"`
}

type MessageTranscription struct {
	Organization     string          `json:"organization"`
	ShortDescription string          `json:"short_description"`
	Event            string          `json:"event"`
	EventStart       *MinuteDateTime `json:"event_start"`
	EventStop        *MinuteDateTime `json:"event_stop"`
	Addresses        []Address       `json:"addresses"`
}

// UnmarshalJSON — event_start/event_stop can arrive as "" (not JSON null)
// from the model. Jackson maps "" to null for LocalDateTime/MinuteDateTime
// fields; a plain `*MinuteDateTime` field would not: encoding/json only
// skips allocating (and calling UnmarshalJSON on) a pointer field for the
// JSON literal null — for "" it allocates a zero-valued MinuteDateTime and
// calls UnmarshalJSON on it, which returns success without setting
// anything, leaving a non-nil pointer to the zero time (0001-01-01
// 00:00). Downstream code treats "pointer set" as "value present" (see
// workflows.Activities.Notify), so that zero time leaks into a bogus
// notification ("... с 01-01-0001 00:00"). Decode event_start/event_stop
// as raw strings first and only build the pointer for a non-empty value,
// so "" behaves exactly like null: the pointer stays nil.
func (m *MessageTranscription) UnmarshalJSON(b []byte) error {
	type alias MessageTranscription
	aux := struct {
		EventStart *string `json:"event_start"`
		EventStop  *string `json:"event_stop"`
		*alias
	}{alias: (*alias)(m)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	m.EventStart = nil
	m.EventStop = nil
	if aux.EventStart != nil && *aux.EventStart != "" {
		t, err := parseLocal(*aux.EventStart)
		if err != nil {
			return fmt.Errorf("event_start: %w", err)
		}
		m.EventStart = &MinuteDateTime{Time: t}
	}
	if aux.EventStop != nil && *aux.EventStop != "" {
		t, err := parseLocal(*aux.EventStop)
		if err != nil {
			return fmt.Errorf("event_stop: %w", err)
		}
		m.EventStop = &MinuteDateTime{Time: t}
	}
	return nil
}

type ParsedMessage struct {
	OriginalMessage      TelegramMessage       `json:"originalMessage"`
	MessageTranscription *MessageTranscription `json:"messageTranscription"`
}
