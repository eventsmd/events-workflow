// Package domain — доменные типы, 1:1 с Java-records. JSON-теги повторяют
// Jackson-имена: @JsonProperty → snake_case, без аннотации → имя поля record.
package domain

import "github.com/google/uuid"

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

type ParsedMessage struct {
	OriginalMessage      TelegramMessage       `json:"originalMessage"`
	MessageTranscription *MessageTranscription `json:"messageTranscription"`
}
