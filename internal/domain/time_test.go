package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func ldt(s string) LocalDateTime {
	t, err := time.Parse("2006-01-02T15:04:05", s)
	if err != nil {
		panic(err)
	}
	return LocalDateTime{t}
}

func TestLocalDateTime_MarshalOmitsZeroSeconds(t *testing.T) {
	b, _ := json.Marshal(ldt("2025-12-01T22:50:00"))
	if string(b) != `"2025-12-01T22:50"` {
		t.Fatalf("got %s", b)
	}
	b, _ = json.Marshal(ldt("2025-12-01T22:50:07"))
	if string(b) != `"2025-12-01T22:50:07"` {
		t.Fatalf("got %s", b)
	}
}

func TestLocalDateTime_UnmarshalTolerant(t *testing.T) {
	for _, in := range []string{
		`"2025-12-01T22:50"`, `"2025-12-01T22:50:07"`, `"2025-12-01T22:50:07.123456"`,
	} {
		var v LocalDateTime
		if err := json.Unmarshal([]byte(in), &v); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if v.Year() != 2025 || v.Minute() != 50 {
			t.Fatalf("%s parsed as %v", in, v)
		}
	}
}

func TestLocalDateTime_String(t *testing.T) {
	if got := ldt("2025-12-01T22:50:00").String(); got != "2025-12-01T22:50" {
		t.Fatal(got)
	}
	if got := ldt("2025-12-01T22:50:07").String(); got != "2025-12-01T22:50:07" {
		t.Fatal(got)
	}
}

func TestMinuteDateTime_Roundtrip(t *testing.T) {
	var v MinuteDateTime
	if err := json.Unmarshal([]byte(`"2025-12-01T22:50:33"`), &v); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(v)
	if string(b) != `"2025-12-01T22:50"` { // minute format on marshal, like @JsonFormat(yyyy-MM-dd'T'HH:mm)
		t.Fatalf("got %s", b)
	}
}

// TestLocalDateTime_UnmarshalStripsTrailingZ — production payloads carry
// `"date":"2026-08-18T12:09:05Z"`. Jackson's LocalDateTimeDeserializer
// accepts that: on the default formatter it drops a single trailing 'Z'
// (UTC indicator) and parses the rest as a local date-time. Without the
// same handling the Temporal workflow input fails to decode entirely
// ("cannot parse local datetime"). Cases below were verified against
// jackson-datatype-jsr310 2.19.4.
func TestLocalDateTime_UnmarshalStripsTrailingZ(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`"2026-08-18T12:09:05Z"`, "2026-08-18T12:09:05"},
		{`"2026-08-18T12:09Z"`, "2026-08-18T12:09"},
		{`"2026-08-18T12:09:05.123Z"`, "2026-08-18T12:09:05.123"},
	} {
		var v LocalDateTime
		if err := json.Unmarshal([]byte(tc.in), &v); err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		want, err := parseLocal(tc.want)
		if err != nil {
			t.Fatal(err)
		}
		if !v.Equal(want) {
			t.Fatalf("%s parsed as %v, want %v", tc.in, v.Time, want)
		}
	}
}

// TestLocalDateTime_UnmarshalRejectsNonJacksonZ — Jackson strips exactly
// one uppercase 'Z' and only when the value is a date-time (charAt(10) ==
// 'T'). Everything else stays a parse error, so a real offset is not
// silently reinterpreted as local time.
func TestLocalDateTime_UnmarshalRejectsNonJacksonZ(t *testing.T) {
	for _, in := range []string{
		`"2026-08-18T12:09:05z"`,  // lowercase
		`"2026-08-18Z"`,           // date only
		`"2026-08-18T12:09:05ZZ"`, // only one Z is dropped
		`"2026-08-18T12:09:05+02:00"`,
		`"Z"`,
	} {
		var v LocalDateTime
		if err := json.Unmarshal([]byte(in), &v); err == nil {
			t.Fatalf("%s: expected an error, got %v", in, v.Time)
		}
	}
}

// TestMinuteDateTime_UnmarshalRejectsTrailingZ — event_start/event_stop
// carried @JsonFormat(pattern = "yyyy-MM-dd'T'HH:mm") in Java, which uses a
// custom formatter and therefore never got Jackson's Z-stripping. The
// leniency must stay confined to TelegramMessage.date.
func TestMinuteDateTime_UnmarshalRejectsTrailingZ(t *testing.T) {
	var v MinuteDateTime
	if err := json.Unmarshal([]byte(`"2026-08-18T12:09:05Z"`), &v); err == nil {
		t.Fatalf("expected an error, got %v", v.Time)
	}
	var m MessageTranscription
	if err := json.Unmarshal([]byte(`{"event_start":"2026-08-18T12:09:05Z"}`), &m); err == nil {
		t.Fatalf("expected an error, got %v", m.EventStart)
	}
}
