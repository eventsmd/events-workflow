package geo

import (
	"context"
	"fmt"

	"events-workflow/internal/domain"
	"events-workflow/internal/store"
)

type Finder interface {
	Find(ctx context.Context, address string) ([]AddressKladr, error)
}

// Picker selects the best variant among several (implementation: ai.AddressPicker).
type Picker interface {
	PickAddress(ctx context.Context, msg domain.ParsedMessage,
		entity *store.AddressEntity, list []AddressKladr) (AddressKladr, error)
}

type Adapter struct {
	finder Finder
	picker Picker
}

func NewAdapter(finder Finder, picker Picker) *Adapter {
	return &Adapter{finder: finder, picker: picker}
}

// Enrich — port of AddressAdapter.enrich: finds variants by
// "{city_original} {street_original}", asks Picker on ambiguity,
// writes found KLADR levels to entity.
func (a *Adapter) Enrich(ctx context.Context, msg domain.ParsedMessage, e *store.AddressEntity) error {
	query := fmt.Sprintf("%s %s", strDeref(e.CityOriginal), strDeref(e.StreetOriginal))
	variants, err := a.finder.Find(ctx, query)
	if err != nil {
		return err
	}
	if len(variants) == 0 {
		return nil
	}
	chosen := variants[0]
	if len(variants) > 1 {
		chosen, err = a.picker.PickAddress(ctx, msg, e, variants)
		if err != nil {
			return err
		}
	}
	if r := chosen.Region; r != nil {
		e.RegionKladr = store.StrPtr(r.Kladr)
		e.RegionName = store.StrPtr(r.Name)
	}
	if c := chosen.City; c != nil {
		e.CityKladr = store.StrPtr(c.Kladr)
		e.CityName = store.StrPtr(c.Name)
	}
	if s := chosen.Street; s != nil {
		e.StreetType = store.StrPtr(s.Type)
		e.StreetKladr = store.StrPtr(s.Kladr)
		e.StreetName = store.StrPtr(s.Name)
	}
	return nil
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
