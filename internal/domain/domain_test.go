package domain

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestTelegramMessage_UnmarshalJacksonPayload(t *testing.T) {
	payload := `{
	  "id": 100, "chat_id": -100200,
	  "from": {"id": 42, "name": "水канал"},
	  "text": "Отключение воды",
	  "date": "2025-12-01T22:50",
	  "reply_to": {"id": 99, "chat_id": -100200},
	  "service_name": "water",
	  "context": {"supplier": "water"}
	}`
	var m TelegramMessage
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatal(err)
	}
	if m.ID != 100 || m.ChatID != -100200 || m.From.Name != "水канал" ||
		m.ReplyTo.ID != 99 || m.ServiceName != "water" ||
		m.Context["supplier"] != "water" {
		t.Fatalf("%+v", m)
	}
}

func TestParsedMessage_FieldNames(t *testing.T) {
	pm := ParsedMessage{OriginalMessage: TelegramMessage{ID: 1, ChatID: 2}}
	b, _ := json.Marshal(pm)
	var raw map[string]json.RawMessage
	json.Unmarshal(b, &raw)
	if _, ok := raw["originalMessage"]; !ok {
		t.Fatalf("want camelCase originalMessage, got %s", b)
	}
	if _, ok := raw["messageTranscription"]; !ok {
		t.Fatalf("want messageTranscription key, got %s", b)
	}
}

func TestMessageTranscription_Unmarshal(t *testing.T) {
	// Формат ответа OpenAI по схеме из промпта MessageParser.
	payload := `{
	  "organization": "Водоканал",
	  "short_description": "отключение",
	  "event": "shutdown",
	  "event_start": "2025-12-02T09:00",
	  "event_stop": "2025-12-02T18:00",
	  "addresses": [{
	    "city": "Тирасполь", "street_type": "ул.", "street": "Ленина",
	    "house": {"numbers": ["1","2"], "ranges": [["10","20"]]}
	  }]
	}`
	var tr MessageTranscription
	if err := json.Unmarshal([]byte(payload), &tr); err != nil {
		t.Fatal(err)
	}
	if tr.Event != "shutdown" || tr.EventStart.Hour() != 9 ||
		tr.Addresses[0].House.Ranges[0][1] != "20" {
		t.Fatalf("%+v", tr)
	}
	if tr.Addresses[0].ID != uuid.Nil {
		t.Fatal("id must stay zero until parser assigns it")
	}
}
