package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/google/uuid"

	"events-workflow/internal/domain"
)

// parsePrompt — дословный порт промпта из MessageParser.java.
const parsePrompt = `Given a text message at %s describing a supply event, generate a JSON object according to the following schema: {"type":"object","properties":{"organization":{"type":"string"},"short_description":{"type":"string"},"event":{"type":"string","enum":["shutdown","resume", "other"]},"event_start":{"type":"string","format":"iso date-time without tz and seconds yyyy-MM-dd'T'HH:mm"},"event_stop":{"type":"string","format":"iso date-time without tz and seconds yyyy-MM-dd'T'HH:mm"},"addresses":{"type":"array","items":{"type":"object","properties":{"city":{"type":"string"},"street_type":{"type":"string","enum":["ул.","пер.","пл."]},"street":{"type":"string"},"house":{"type":"object","properties":{"numbers":{"type":"array","items":{"type":"string"}},"ranges":{"type":"array","items":{"type":"array","items":{"type":"string"}}}}}}}}}} The JSON output must accurately reflect the details from the message. Response - ONLY JSON. Ignore named places, we need only addresses. Message starts with organization name usually. Be accurate with addresses! Message could have no address. In case you can't recognize what has happened - return just null.`

var codeFence = regexp.MustCompile("```(?:json)?")

type MessageParser struct{ client *Client }

func NewMessageParser(c *Client) *MessageParser { return &MessageParser{c} }

// ВНИМАНИЕ: в Java-промпте первый "%s" (после "at ") не подставляется —
// строка форматируется только в хвостовой части. Воспроизводим:
// полный промпт = parsePrompt (как есть, с литеральным %s)
// + "\nMessage at {t}:\n{message}".
func (p *MessageParser) Parse(ctx context.Context, t domain.LocalDateTime, message string) (*domain.MessageTranscription, error) {
	prompt := parsePrompt + fmt.Sprintf("\nMessage at %s:\n%s", t, message)
	raw, err := p.client.Chat(ctx, prompt)
	if err != nil {
		return nil, err
	}
	cleaned := codeFence.ReplaceAllString(raw, "")
	var tr *domain.MessageTranscription
	if err := json.Unmarshal([]byte(cleaned), &tr); err != nil {
		return nil, fmt.Errorf("parse transcription %q: %w", cleaned, err)
	}
	if tr == nil {
		return nil, nil
	}
	// Порт компакт-конструктора Address: id == null ⇒ random UUID.
	for i := range tr.Addresses {
		if tr.Addresses[i].ID == uuid.Nil {
			tr.Addresses[i].ID = uuid.New()
		}
	}
	return tr, nil
}
