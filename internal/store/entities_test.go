package store

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"events-workflow/internal/domain"
)

func TestAddressEntityFrom_JoinsHouses(t *testing.T) {
	a := domain.Address{
		ID: uuid.New(), City: "Тирасполь", Street: "Ленина", StreetType: "ул.",
		House: &domain.House{Numbers: []string{"1", "2"}, Ranges: [][]string{{"10", "20"}, {"30", "40"}}},
	}
	e := AddressEntityFrom(a, domain.MessageReference{ID: 5, ChatID: 6})
	if e.ID != a.ID || e.MessageID != 5 || e.ChatID != 6 {
		t.Fatalf("%+v", e)
	}
	if *e.HouseNumbers != "1,2" {
		t.Fatal(*e.HouseNumbers)
	}
	if *e.HouseRanges != "10-20;30-40" {
		t.Fatal(*e.HouseRanges)
	}
}

func TestAddressEntityFrom_NilHouse(t *testing.T) {
	e := AddressEntityFrom(domain.Address{ID: uuid.New()}, domain.MessageReference{})
	if e.HouseNumbers != nil || e.HouseRanges != nil {
		t.Fatal("expected nil house fields")
	}
}

func TestFormatHouses(t *testing.T) {
	e := &AddressEntity{HouseNumbers: StrPtr("1,2"), HouseRanges: StrPtr("10-20")}
	if got := e.FormatHouses(); got != "д. 1, 2, 10-20" {
		t.Fatal(got)
	}
	if got := (&AddressEntity{}).FormatHouses(); got != "" {
		t.Fatal(got)
	}
}

func TestHouses(t *testing.T) {
	e := &AddressEntity{HouseNumbers: StrPtr("1,2"), HouseRanges: StrPtr("10-20;30-40")}
	got := e.Houses()
	want := []string{"1", "2", "10-20", "30-40"}
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%v", got)
		}
	}
}

func TestTelegramMessageEntity_Roundtrip(t *testing.T) {
	inc := uuid.New()
	msg := domain.TelegramMessage{
		ID: 1, ChatID: 2,
		From:    &domain.User{ID: 3, Name: "n"},
		Text:    "text",
		Date:    domain.LocalDateTime{Time: time.Date(2025, 12, 1, 22, 50, 0, 0, time.UTC)},
		ReplyTo: &domain.MessageReference{ID: 9, ChatID: 2},
		Context: map[string]string{"supplier": "water"},
	}
	e := TelegramMessageEntityFrom(msg, inc)
	if *e.FromID != 3 || *e.ReplyToID != 9 || *e.IncidentID != inc {
		t.Fatalf("%+v", e)
	}
	back := e.ToDomain()
	// service_name null ⇒ fallback to context["supplier"] (behavior of toDomain() in Java)
	if back.ServiceName != "water" {
		t.Fatal(back.ServiceName)
	}
	if back.From.ID != 3 || back.ReplyTo.ID != 9 {
		t.Fatalf("%+v", back)
	}
}

func TestTranscribeEntityFrom(t *testing.T) {
	start := domain.MinuteDateTime{Time: time.Date(2025, 12, 2, 9, 0, 0, 0, time.UTC)}
	pm := domain.ParsedMessage{
		OriginalMessage: domain.TelegramMessage{ID: 1, ChatID: 2},
		MessageTranscription: &domain.MessageTranscription{
			Organization: "org", ShortDescription: "desc", Event: "shutdown",
			EventStart: &start,
		},
	}
	e := TranscribeEntityFrom(pm)
	if e.ID != 1 || e.ChatID != 2 || *e.Event != "shutdown" ||
		!e.EventStart.Equal(start.Time) || e.EventStop != nil {
		t.Fatalf("%+v", e)
	}
}

// Additional test cases ported from Java tests

// AddressEntityTest cases

func TestAddressEntityFrom_MapBasicFields(t *testing.T) {
	id := uuid.New()
	address := domain.Address{ID: id, City: "Кишинёв", Street: "Пушкина", StreetType: "ул.", House: nil}
	ref := domain.MessageReference{ID: 1, ChatID: 2}

	entity := AddressEntityFrom(address, ref)

	if entity.ID != id {
		t.Fatalf("ID mismatch: %v != %v", entity.ID, id)
	}
	if *entity.CityOriginal != "Кишинёв" {
		t.Fatalf("CityOriginal: %v", entity.CityOriginal)
	}
	if *entity.StreetOriginal != "Пушкина" {
		t.Fatalf("StreetOriginal: %v", entity.StreetOriginal)
	}
	if *entity.StreetTypeOriginal != "ул." {
		t.Fatalf("StreetTypeOriginal: %v", entity.StreetTypeOriginal)
	}
	if entity.MessageID != 1 {
		t.Fatalf("MessageID: %v", entity.MessageID)
	}
	if entity.ChatID != 2 {
		t.Fatalf("ChatID: %v", entity.ChatID)
	}
	if entity.HouseNumbers != nil {
		t.Fatal("HouseNumbers should be nil")
	}
	if entity.HouseRanges != nil {
		t.Fatal("HouseRanges should be nil")
	}
}

func TestAddressEntityFrom_MapHouseNumbersOnly(t *testing.T) {
	address := domain.Address{
		ID: uuid.New(), City: "Кишинёв", Street: "Пушкина", StreetType: "ул.",
		House: &domain.House{Numbers: []string{"1", "2", "3А"}, Ranges: nil},
	}
	ref := domain.MessageReference{ID: 1, ChatID: 2}

	entity := AddressEntityFrom(address, ref)

	if *entity.HouseNumbers != "1,2,3А" {
		t.Fatalf("HouseNumbers: %v", entity.HouseNumbers)
	}
	if entity.HouseRanges != nil {
		t.Fatal("HouseRanges should be nil")
	}
}

func TestAddressEntityFrom_MapHouseRangesOnly(t *testing.T) {
	address := domain.Address{
		ID: uuid.New(), City: "Кишинёв", Street: "Пушкина", StreetType: "ул.",
		House: &domain.House{Numbers: nil, Ranges: [][]string{{"1", "10"}, {"20", "30"}}},
	}
	ref := domain.MessageReference{ID: 1, ChatID: 2}

	entity := AddressEntityFrom(address, ref)

	if entity.HouseNumbers != nil {
		t.Fatal("HouseNumbers should be nil")
	}
	if *entity.HouseRanges != "1-10;20-30" {
		t.Fatalf("HouseRanges: %v", entity.HouseRanges)
	}
}

func TestAddressEntityFrom_HandleEmptyHouseLists(t *testing.T) {
	address := domain.Address{
		ID: uuid.New(), City: "Кишинёв", Street: "Пушкина", StreetType: "ул.",
		House: &domain.House{Numbers: []string{}, Ranges: [][]string{}},
	}
	ref := domain.MessageReference{ID: 1, ChatID: 2}

	entity := AddressEntityFrom(address, ref)

	if entity.HouseNumbers != nil {
		t.Fatal("HouseNumbers should be nil")
	}
	if entity.HouseRanges != nil {
		t.Fatal("HouseRanges should be nil")
	}
}

func TestAddressEntityFrom_HandleBothNumbersAndRanges(t *testing.T) {
	address := domain.Address{
		ID: uuid.New(), City: "Кишинёв", Street: "Пушкина", StreetType: "ул.",
		House: &domain.House{Numbers: []string{"5", "7"}, Ranges: [][]string{{"10", "20"}}},
	}
	ref := domain.MessageReference{ID: 1, ChatID: 2}

	entity := AddressEntityFrom(address, ref)

	if *entity.HouseNumbers != "5,7" {
		t.Fatalf("HouseNumbers: %v", entity.HouseNumbers)
	}
	if *entity.HouseRanges != "10-20" {
		t.Fatalf("HouseRanges: %v", entity.HouseRanges)
	}
}

// AddressEntityHouseDisplayTest cases

func TestFormatHouses_NumbersOnly(t *testing.T) {
	entity := &AddressEntity{HouseNumbers: StrPtr("1,2,3")}
	if got := entity.FormatHouses(); got != "д. 1, 2, 3" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatHouses_RangesOnly(t *testing.T) {
	entity := &AddressEntity{HouseRanges: StrPtr("1-10;20-30")}
	if got := entity.FormatHouses(); got != "д. 1-10, 20-30" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatHouses_NumbersAndRanges(t *testing.T) {
	entity := &AddressEntity{
		HouseNumbers: StrPtr("1,2,3"),
		HouseRanges:  StrPtr("5-10;20-30"),
	}
	if got := entity.FormatHouses(); got != "д. 1, 2, 3, 5-10, 20-30" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatHouses_NoHouses(t *testing.T) {
	entity := &AddressEntity{}
	if got := entity.FormatHouses(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatHouses_NullNumbers(t *testing.T) {
	entity := &AddressEntity{HouseRanges: StrPtr("1-10")}
	if got := entity.FormatHouses(); got != "д. 1-10" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatHouses_NullRanges(t *testing.T) {
	entity := &AddressEntity{HouseNumbers: StrPtr("5,7")}
	if got := entity.FormatHouses(); got != "д. 5, 7" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatHouses_SingleNumber(t *testing.T) {
	entity := &AddressEntity{HouseNumbers: StrPtr("42")}
	if got := entity.FormatHouses(); got != "д. 42" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatHouses_SingleRange(t *testing.T) {
	entity := &AddressEntity{HouseRanges: StrPtr("1-100")}
	if got := entity.FormatHouses(); got != "д. 1-100" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatHouses_EmptyStrings(t *testing.T) {
	entity := &AddressEntity{
		HouseNumbers: StrPtr(""),
		HouseRanges:  StrPtr(""),
	}
	if got := entity.FormatHouses(); got != "" {
		t.Fatalf("got %q", got)
	}
}

// TelegramMessageEntityTest cases

func TestTelegramMessageEntity_FromMapAllFields(t *testing.T) {
	user := domain.User{ID: 1, Name: "TestUser"}
	replyTo := domain.MessageReference{ID: 2, ChatID: 10}
	context := map[string]string{"supplier": "water"}
	date := domain.LocalDateTime{Time: time.Date(2025, 12, 1, 10, 0, 0, 0, time.UTC)}
	incidentID := uuid.New()

	message := domain.TelegramMessage{
		ID: 100, ChatID: 200, From: &user, Text: "text", Date: date,
		ReplyTo: &replyTo, ServiceName: "water", Context: context,
	}

	entity := TelegramMessageEntityFrom(message, incidentID)

	if entity.ID != 100 {
		t.Fatalf("ID: %v", entity.ID)
	}
	if entity.ChatID != 200 {
		t.Fatalf("ChatID: %v", entity.ChatID)
	}
	if entity.Text != "text" {
		t.Fatalf("Text: %v", entity.Text)
	}
	if entity.Date != date.Time {
		t.Fatalf("Date: %v", entity.Date)
	}
	if *entity.ServiceName != "water" {
		t.Fatalf("ServiceName: %v", entity.ServiceName)
	}
	if *entity.FromID != 1 {
		t.Fatalf("FromID: %v", entity.FromID)
	}
	if *entity.FromName != "TestUser" {
		t.Fatalf("FromName: %v", entity.FromName)
	}
	if *entity.ReplyToID != 2 {
		t.Fatalf("ReplyToID: %v", entity.ReplyToID)
	}
	if *entity.ReplyToChatID != 10 {
		t.Fatalf("ReplyToChatID: %v", entity.ReplyToChatID)
	}
	if *entity.IncidentID != incidentID {
		t.Fatalf("IncidentID: %v", entity.IncidentID)
	}
	if len(entity.Context) != len(context) {
		t.Fatalf("Context length: %v", entity.Context)
	}
	for k, v := range context {
		if entity.Context[k] != v {
			t.Fatalf("Context[%s]: %v", k, entity.Context[k])
		}
	}
}

func TestTelegramMessageEntity_HandleNullFromAndReplyTo(t *testing.T) {
	date := domain.LocalDateTime{Time: time.Now()}
	message := domain.TelegramMessage{
		ID: 1, ChatID: 2, From: nil, Text: "text", Date: date,
		ReplyTo: nil, ServiceName: "", Context: nil,
	}

	entity := TelegramMessageEntityFrom(message, uuid.New())

	if entity.FromID != nil {
		t.Fatal("FromID should be nil")
	}
	if entity.FromName != nil {
		t.Fatal("FromName should be nil")
	}
	if entity.ReplyToID != nil {
		t.Fatal("ReplyToID should be nil")
	}
	if entity.ReplyToChatID != nil {
		t.Fatal("ReplyToChatID should be nil")
	}
}

func TestTelegramMessageEntity_ToDomainFallbackContextSupplier(t *testing.T) {
	entity := &TelegramMessageEntity{
		ID: 1, ChatID: 2, Text: "text",
		Date: time.Now(), ServiceName: nil,
		Context: map[string]string{"supplier": "electricity"},
	}

	domain := entity.ToDomain()

	if domain.ServiceName != "electricity" {
		t.Fatalf("ServiceName: %v", domain.ServiceName)
	}
}

func TestTelegramMessageEntity_ToDomainNullContextWhenServiceNameNull(t *testing.T) {
	entity := &TelegramMessageEntity{
		ID: 1, ChatID: 2, Text: "text",
		Date: time.Now(), ServiceName: nil,
		Context: nil,
	}

	domain := entity.ToDomain()

	if domain.ServiceName != "" {
		t.Fatalf("ServiceName should be empty: %v", domain.ServiceName)
	}
}

func TestTelegramMessageEntity_ToDomainHandleNullFromFields(t *testing.T) {
	entity := &TelegramMessageEntity{
		ID: 1, ChatID: 2, Text: "text",
		Date: time.Now(), Context: map[string]string{},
	}

	domain := entity.ToDomain()

	if domain.From != nil {
		t.Fatal("From should be nil")
	}
	if domain.ReplyTo != nil {
		t.Fatal("ReplyTo should be nil")
	}
}

// TelegramMessageTranscribeEntityTest cases

func TestTranscribeEntityFrom_MapAllFields(t *testing.T) {
	start := domain.MinuteDateTime{Time: time.Date(2025, 12, 1, 10, 0, 0, 0, time.UTC)}
	stop := domain.MinuteDateTime{Time: time.Date(2025, 12, 1, 18, 0, 0, 0, time.UTC)}

	message := domain.TelegramMessage{ID: 100, ChatID: 200}
	transcription := domain.MessageTranscription{
		Organization: "SA Apă-Canal", ShortDescription: "Отключение воды", Event: "shutdown",
		EventStart: &start, EventStop: &stop,
	}
	parsed := domain.ParsedMessage{
		OriginalMessage: message, MessageTranscription: &transcription,
	}

	entity := TranscribeEntityFrom(parsed)

	if entity.ID != 100 {
		t.Fatalf("ID: %v", entity.ID)
	}
	if entity.ChatID != 200 {
		t.Fatalf("ChatID: %v", entity.ChatID)
	}
	if *entity.Organization != "SA Apă-Canal" {
		t.Fatalf("Organization: %v", entity.Organization)
	}
	if *entity.Description != "Отключение воды" {
		t.Fatalf("Description: %v", entity.Description)
	}
	if *entity.Event != "shutdown" {
		t.Fatalf("Event: %v", entity.Event)
	}
	if !entity.EventStart.Equal(start.Time) {
		t.Fatalf("EventStart: %v", entity.EventStart)
	}
	if !entity.EventStop.Equal(stop.Time) {
		t.Fatalf("EventStop: %v", entity.EventStop)
	}
}

func TestStrPtr(t *testing.T) {
	// StrPtr should return nil for empty string
	if StrPtr("") != nil {
		t.Fatal("StrPtr(\"\") should be nil")
	}
	// StrPtr should return pointer for non-empty string
	s := "test"
	if got := StrPtr(s); got == nil || *got != s {
		t.Fatal("StrPtr(\"test\") failed")
	}
}

func TestHouses_OnlyNumbers(t *testing.T) {
	e := &AddressEntity{HouseNumbers: StrPtr("1,2,3")}
	got := e.Houses()
	want := []string{"1", "2", "3"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestHouses_OnlyRanges(t *testing.T) {
	e := &AddressEntity{HouseRanges: StrPtr("1-10;20-30")}
	got := e.Houses()
	want := []string{"1-10", "20-30"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestHouses_Empty(t *testing.T) {
	e := &AddressEntity{}
	got := e.Houses()
	if len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
	// Java's houses() starts with new ArrayList<String>() and never returns
	// null; a nil Go slice here would marshal as JSON null downstream
	// (events.EventAddress.Houses) instead of Java's [], breaking wire
	// parity. Regression prevention for code review round 2.
	if got == nil {
		t.Fatal("Houses() must return a non-nil empty slice, got nil (breaks JSON [] parity with Java ArrayList)")
	}
}
