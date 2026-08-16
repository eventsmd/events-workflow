package ai

import (
	"context"
	"strings"
	"testing"

	"events-workflow/internal/domain"
	"events-workflow/internal/geo"
	"events-workflow/internal/store"
)

// Compile-time assertion that AddressPicker implements geo.Picker
var _ geo.Picker = (*AddressPicker)(nil)

func TestPickAddress_ReturnsIndexedVariant(t *testing.T) {
	var prompt string
	picker := NewAddressPicker(fakeOpenAI(t, " 1 ", &prompt)) // answer with spaces — Java does trim()
	list := []geo.AddressKladr{
		{FullAddress: "г. Тирасполь, ул. Ленина"},
		{FullAddress: "с. Ближний Хутор, ул. Ленина"},
	}
	msg := domain.ParsedMessage{OriginalMessage: domain.TelegramMessage{
		From: &domain.User{Name: "Водоканал"}, Text: "отключение по ул. Ленина"}}
	e := &store.AddressEntity{CityOriginal: store.StrPtr("Тирасполь"),
		StreetOriginal: store.StrPtr("Ленина")}

	got, err := picker.PickAddress(context.Background(), msg, e, list)
	if err != nil {
		t.Fatal(err)
	}
	if got.FullAddress != "с. Ближний Хутор, ул. Ленина" {
		t.Fatalf("%+v", got)
	}
	// variant numbering starts at zero, like AtomicInteger(0) in Java
	if !strings.Contains(prompt, "0. г. Тирасполь, ул. Ленина\n") ||
		!strings.Contains(prompt, "1. с. Ближний Хутор, ул. Ленина\n") {
		t.Fatalf("prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, "The original address is: \n'Тирасполь Ленина'") {
		t.Fatalf("prompt:\n%s", prompt)
	}
	if !strings.Contains(prompt, "(Водоканал) отключение по ул. Ленина") {
		t.Fatalf("prompt:\n%s", prompt)
	}
}

func TestPickAddress_NonNumericAnswer(t *testing.T) {
	picker := NewAddressPicker(fakeOpenAI(t, "не знаю", nil))
	_, err := picker.PickAddress(context.Background(), domain.ParsedMessage{
		OriginalMessage: domain.TelegramMessage{From: &domain.User{}}},
		&store.AddressEntity{}, []geo.AddressKladr{{}, {}})
	if err == nil {
		t.Fatal("expected parse error") // Java throws NumberFormatException → activity retry
	}
}
