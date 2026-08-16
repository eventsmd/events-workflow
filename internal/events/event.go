// Package events — публикация UtilityEvent в NATS JetStream.
// JSON и сабжекты идентичны Java-версии (NatsEventPublisher).
package events

import (
	"strings"
	"time"

	"events-workflow/internal/domain"
)

type KladrRef struct {
	Name  string `json:"name,omitempty"`
	Kladr string `json:"kladr,omitempty"`
	Type  string `json:"type,omitempty"`
}

type EventAddress struct {
	Region *KladrRef `json:"region,omitempty"`
	City   *KladrRef `json:"city,omitempty"`
	Street *KladrRef `json:"street,omitempty"`
	Houses []string  `json:"houses,omitempty"`
}

type EventSource struct {
	MessageID int64 `json:"messageId"`
	ChatID    int64 `json:"chatId"`
}

type UtilityEvent struct {
	IncidentID   string                 `json:"incidentId,omitempty"`
	Supplier     string                 `json:"supplier,omitempty"`
	Event        string                 `json:"event,omitempty"`
	Organization string                 `json:"organization,omitempty"`
	Description  string                 `json:"description,omitempty"`
	EventStart   *domain.MinuteDateTime `json:"eventStart,omitempty"`
	EventStop    *domain.MinuteDateTime `json:"eventStop,omitempty"`
	PublishedAt  time.Time              `json:"publishedAt"`
	Source       *EventSource           `json:"source,omitempty"`
	Addresses    []EventAddress         `json:"addresses,omitempty"`
}

// SanitizeToken — порт NatsEventPublisher.sanitizeToken: пробельные и
// служебные для сабжектов символы (. * > /) заменяются на _, подряд идущие
// схлопываются, крайние обрезаются; пустой результат — "_".
func SanitizeToken(s string) string {
	if s == "" {
		return "_"
	}
	var b strings.Builder
	prevUnderscore := false
	for _, c := range s {
		switch c {
		case ' ', '\t', '\n', '\r', '.', '*', '>', '/':
			if !prevUnderscore {
				b.WriteByte('_')
				prevUnderscore = true
			}
		default:
			b.WriteRune(c)
			prevUnderscore = false
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "_"
	}
	return out
}

func SubjectFor(prefix, supplier, event string) string {
	return prefix + "." + SanitizeToken(supplier) + "." + SanitizeToken(event)
}
