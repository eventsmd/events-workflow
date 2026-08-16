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
	// Houses intentionally has no omitempty: Jackson's NON_NULL only omits a
	// null List — a non-null empty List still serializes as []. Go's
	// omitempty conflates nil and len==0, so it must stay off here. Callers
	// that never intend to include this field should leave it nil (which
	// still marshals as literal null, unlike Java's key omission); in
	// practice every real caller (message-transcription mapping) builds a
	// non-nil slice, so this only matters for hand-built empty events.
	Houses []string `json:"houses"`
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
	// Addresses: same NON_NULL-vs-omitempty rationale as EventAddress.Houses
	// above — a non-null empty List in Java still serializes as [].
	Addresses []EventAddress `json:"addresses"`
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
