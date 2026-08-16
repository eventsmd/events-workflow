package geo

import (
	"context"
	"testing"

	"events-workflow/internal/domain"
	"events-workflow/internal/store"
)

type fakeFinder struct{ result []AddressKladr }

func (f fakeFinder) Find(ctx context.Context, address string) ([]AddressKladr, error) {
	return f.result, nil
}

type fakePicker struct{ picked AddressKladr; called bool }

func (f *fakePicker) PickAddress(ctx context.Context, msg domain.ParsedMessage,
	e *store.AddressEntity, list []AddressKladr) (AddressKladr, error) {
	f.called = true
	return f.picked, nil
}

func variant(street string) AddressKladr {
	return AddressKladr{
		Region: &KladrEntity{Kladr: "r-kladr", Name: "Region"},
		City:   &KladrEntity{Kladr: "c-kladr", Name: "City"},
		Street: &KladrEntity{Kladr: "s-kladr", Name: street, Type: "ул."},
	}
}

func TestEnrich_SingleVariant_NoPicker(t *testing.T) {
	p := &fakePicker{}
	a := NewAdapter(fakeFinder{[]AddressKladr{variant("Ленина")}}, p)
	e := &store.AddressEntity{CityOriginal: store.StrPtr("Тирасполь"), StreetOriginal: store.StrPtr("Ленина")}
	if err := a.Enrich(context.Background(), domain.ParsedMessage{}, e); err != nil {
		t.Fatal(err)
	}
	if p.called {
		t.Fatal("picker must not be called for single variant")
	}
	if *e.StreetName != "Ленина" || *e.StreetKladr != "s-kladr" || *e.StreetType != "ул." ||
		*e.CityKladr != "c-kladr" || *e.RegionKladr != "r-kladr" {
		t.Fatalf("%+v", e)
	}
}

func TestEnrich_MultipleVariants_UsesPicker(t *testing.T) {
	p := &fakePicker{picked: variant("Правильная")}
	a := NewAdapter(fakeFinder{[]AddressKladr{variant("A"), variant("B")}}, p)
	e := &store.AddressEntity{}
	if err := a.Enrich(context.Background(), domain.ParsedMessage{}, e); err != nil {
		t.Fatal(err)
	}
	if !p.called || *e.StreetName != "Правильная" {
		t.Fatalf("called=%v %+v", p.called, e)
	}
}

func TestEnrich_NoVariants_NoOp(t *testing.T) {
	a := NewAdapter(fakeFinder{nil}, &fakePicker{})
	e := &store.AddressEntity{}
	if err := a.Enrich(context.Background(), domain.ParsedMessage{}, e); err != nil {
		t.Fatal(err)
	}
	if e.StreetName != nil {
		t.Fatal("must stay empty")
	}
}

func TestEnrich_PartialKladr_OnlyPresentLevels(t *testing.T) {
	v := AddressKladr{City: &KladrEntity{Kladr: "c", Name: "City"}} // без region/street
	a := NewAdapter(fakeFinder{[]AddressKladr{v}}, &fakePicker{})
	e := &store.AddressEntity{}
	_ = a.Enrich(context.Background(), domain.ParsedMessage{}, e)
	if e.RegionKladr != nil || e.StreetKladr != nil || *e.CityKladr != "c" {
		t.Fatalf("%+v", e)
	}
}

func TestEnrich_EmptyList_NoOp(t *testing.T) {
	a := NewAdapter(fakeFinder{[]AddressKladr{}}, &fakePicker{})
	e := &store.AddressEntity{}
	if err := a.Enrich(context.Background(), domain.ParsedMessage{}, e); err != nil {
		t.Fatal(err)
	}
	if e.StreetKladr != nil || e.CityKladr != nil || e.RegionKladr != nil {
		t.Fatal("must stay empty")
	}
}
