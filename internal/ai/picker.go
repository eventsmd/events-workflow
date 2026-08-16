package ai

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"events-workflow/internal/domain"
	"events-workflow/internal/geo"
	"events-workflow/internal/store"
)

type AddressPicker struct{ client *Client }

func NewAddressPicker(c *Client) *AddressPicker { return &AddressPicker{c} }

// PickAddress — дословный порт AddressPicker.pickAddress (Java %n → \n).
func (p *AddressPicker) PickAddress(ctx context.Context, msg domain.ParsedMessage,
	entity *store.AddressEntity, list []geo.AddressKladr) (geo.AddressKladr, error) {

	var addresses strings.Builder
	for i, k := range list {
		fmt.Fprintf(&addresses, "%d. %s\n", i, k.FullAddress)
	}
	original := fmt.Sprintf("%s %s", strDeref(entity.CityOriginal), strDeref(entity.StreetOriginal))
	var fromName, text string
	if msg.OriginalMessage.From != nil {
		fromName = msg.OriginalMessage.From.Name
	}
	text = msg.OriginalMessage.Text

	prompt := fmt.Sprintf("Select the address variant that best matches the original address based on semantic similarity and context of request. The original address is: \n'%s'. The variants are: %s . Respond only with the number of the most similar variant, in int64 format!\nIt was fetched from the message: (%s) %s",
		original, addresses.String(), fromName, text)

	answer, err := p.client.Chat(ctx, prompt)
	if err != nil {
		return geo.AddressKladr{}, err
	}
	idx, err := strconv.Atoi(strings.TrimSpace(answer))
	if err != nil {
		return geo.AddressKladr{}, fmt.Errorf("picker answer %q: %w", answer, err)
	}
	if idx < 0 || idx >= len(list) {
		return geo.AddressKladr{}, fmt.Errorf("picker index %d out of range %d", idx, len(list))
	}
	return list[idx], nil
}

func strDeref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
