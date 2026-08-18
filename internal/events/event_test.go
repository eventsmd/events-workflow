package events

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"events-workflow/internal/domain"
	"events-workflow/internal/store"
)

func TestSanitizeToken(t *testing.T) {
	cases := map[string]string{
		"water":         "water",
		"hot water":     "hot_water",
		"a.b*c>d/e":     "a_b_c_d_e",
		"  spaced  ":    "spaced",
		"":              "_",
		"...":           "_",
		"a  b":          "a_b", // collapse consecutive underscores
		"tab\tand\nnew": "tab_and_new",
	}
	for in, want := range cases {
		if got := SanitizeToken(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestSubjectFor(t *testing.T) {
	if got := SubjectFor("pmr.utility.event", "hot water", "shutdown"); got != "pmr.utility.event.hot_water.shutdown" {
		t.Fatal(got)
	}
}

func TestUtilityEvent_JSON(t *testing.T) {
	start := domain.MinuteDateTime{Time: time.Date(2025, 12, 2, 9, 0, 0, 0, time.UTC)}
	ev := UtilityEvent{
		IncidentID:  "11111111-2222-3333-4444-555555555555",
		Supplier:    "water",
		Event:       "shutdown",
		Description: "desc",
		EventStart:  &start,
		PublishedAt: time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC),
		Source:      &EventSource{MessageID: 1, ChatID: 2},
		Addresses: []EventAddress{{
			City:   &KladrRef{Name: "Тирасполь", Kladr: "123-01.001-02.002-00.000-00.000", Type: "г."},
			Houses: []string{"1", "10-20"},
		}},
	}
	b, _ := json.Marshal(ev)
	s := string(b)
	for _, want := range []string{
		`"incidentId":"11111111-2222-3333-4444-555555555555"`,
		`"eventStart":"2025-12-02T09:00"`,
		`"source":{"messageId":1,"chatId":2}`,
		`"houses":["1","10-20"]`,
		`"publishedAt":"2026-08-16T12:00:00Z"`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
	// NON_NULL: empty fields are omitted
	for _, absent := range []string{`"organization"`, `"eventStop"`, `"region"`, `"street"`} {
		if strings.Contains(s, absent) {
			t.Fatalf("unexpected %s in %s", absent, s)
		}
	}
}

// Ported from UtilityEventJsonTest.serializesCamelCaseWithFormattedDates:
// fully populated event with all three KladrRef and organization.
func TestUtilityEvent_JSON_FullyPopulated(t *testing.T) {
	start := domain.MinuteDateTime{Time: time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)}
	stop := domain.MinuteDateTime{Time: time.Date(2026, 6, 21, 18, 0, 0, 0, time.UTC)}
	ev := UtilityEvent{
		IncidentID:   "inc-1",
		Supplier:     "water",
		Event:        "shutdown",
		Organization: "SA Apă-Canal",
		Description:  "Отключение воды",
		EventStart:   &start,
		EventStop:    &stop,
		PublishedAt:  time.Date(2026, 6, 21, 7, 0, 0, 0, time.UTC),
		Source:       &EventSource{MessageID: 123, ChatID: -100123},
		Addresses: []EventAddress{{
			Region: &KladrRef{Name: "Кишинёв", Kladr: "001", Type: "г"},
			City:   &KladrRef{Name: "Кишинёв", Kladr: "001-01", Type: "г"},
			Street: &KladrRef{Name: "Пушкина", Kladr: "001-01.001", Type: "ул"},
			Houses: []string{"1", "3", "5-9"},
		}},
	}
	b, _ := json.Marshal(ev)
	s := string(b)
	for _, want := range []string{
		`"incidentId":"inc-1"`,
		`"supplier":"water"`,
		`"eventStart":"2026-06-21T10:00"`,
		`"eventStop":"2026-06-21T18:00"`,
		`"publishedAt":"2026-06-21T07:00:00Z"`,
		`"houses":["1","3","5-9"]`,
		`"messageId":123`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
}

// Ported from UtilityEventJsonTest.omitsNullEventStop.
func TestUtilityEvent_JSON_OmitsNullEventStop(t *testing.T) {
	start := domain.MinuteDateTime{Time: time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)}
	ev := UtilityEvent{
		IncidentID:   "inc-2",
		Supplier:     "electricity",
		Event:        "resume",
		Organization: "org",
		Description:  "desc",
		EventStart:   &start,
		PublishedAt:  time.Date(2026, 6, 21, 7, 0, 0, 0, time.UTC),
		Source:       &EventSource{MessageID: 1, ChatID: 2},
	}
	b, _ := json.Marshal(ev)
	s := string(b)
	if strings.Contains(s, "eventStop") {
		t.Fatalf("unexpected eventStop in %s", s)
	}
}

// Regression from code review round 1: Jackson's @JsonInclude(NON_NULL) only omits
// a null List — a non-null empty List still serializes as []. Go's
// omitempty conflates nil and len==0, so Houses/Addresses must not carry
// omitempty. Real callers (message-transcription mapping) always build
// non-nil (possibly empty) slices, e.g. Task 10's activity code.
func TestUtilityEvent_JSON_EmptyHousesAndAddressesSerializeAsEmptyArrays(t *testing.T) {
	ev := UtilityEvent{
		IncidentID:  "inc-3",
		Supplier:    "water",
		Event:       "shutdown",
		PublishedAt: time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC),
		Addresses:   []EventAddress{}, // non-nil, no rows
	}
	b, _ := json.Marshal(ev)
	s := string(b)
	if !strings.Contains(s, `"addresses":[]`) {
		t.Fatalf("want addresses:[] for empty non-nil slice, got %s", s)
	}

	addr := EventAddress{Houses: []string{}} // non-nil, no house numbers/ranges
	hb, _ := json.Marshal(addr)
	hs := string(hb)
	if !strings.Contains(hs, `"houses":[]`) {
		t.Fatalf("want houses:[] for empty non-nil slice, got %s", hs)
	}
}

// Regression from code review round 2: store.AddressEntity.Houses() must return a
// non-nil empty slice (matching Java's new ArrayList<String>()) so an
// EventAddress built from a house-less address still marshals as
// "houses":[] rather than "houses":null.
func TestUtilityEvent_JSON_HousesFromEmptyAddressEntitySerializesAsEmptyArray(t *testing.T) {
	addr := EventAddress{Houses: (&store.AddressEntity{}).Houses()}
	b, _ := json.Marshal(addr)
	s := string(b)
	if !strings.Contains(s, `"houses":[]`) {
		t.Fatalf("want houses:[] from house-less AddressEntity, got %s", s)
	}
}

// Ported from NatsEventPublisherTest.sanitizesIllegalTokenChars —
// one subject-reserved character at a time.
func TestSanitizeToken_IndividualIllegalChars(t *testing.T) {
	cases := map[string]string{
		"a b": "a_b",
		"a.b": "a_b",
		"a*b": "a_b",
		"a>b": "a_b",
		"a/b": "a_b",
		".x.": "x",
	}
	for in, want := range cases {
		if got := SanitizeToken(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}
