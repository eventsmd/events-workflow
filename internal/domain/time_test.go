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
	if string(b) != `"2025-12-01T22:50"` { // минутный формат при маршале, как @JsonFormat(yyyy-MM-dd'T'HH:mm)
		t.Fatalf("got %s", b)
	}
}
