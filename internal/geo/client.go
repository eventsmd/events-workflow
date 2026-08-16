// Package geo — клиент geo-сервиса (GET {GEO_BASE_URL}/parse?address=…)
// и обогащение адресов KLADR-кодами. Порт AddressApi/AddressAdapter.
package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type KladrEntity struct {
	Kladr        string `json:"kladr"`
	Name         string `json:"name"`
	NameWithType string `json:"name_with_type"`
	Type         string `json:"type"`
	TypeFull     string `json:"type_full"`
}

type AddressKladr struct {
	Country     string       `json:"country"`
	FullAddress string       `json:"full_address"`
	House       string       `json:"house"`
	Kladr       string       `json:"kladr"`
	Region      *KladrEntity `json:"region"`
	City        *KladrEntity `json:"city"`
	Street      *KladrEntity `json:"street"`
}

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: 30 * time.Second}}
}

// encodeQueryValue — url.QueryEscape encodes spaces as '+' (application/
// x-www-form-urlencoded); Spring's UriComponentsBuilder encoded them as
// %20. A geo backend that doesn't form-decode the query string would see
// literal '+' characters (e.g. "Тирасполь+Ленина") and silently fail to
// match, leaving every address un-enriched.
func encodeQueryValue(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func (c *Client) Find(ctx context.Context, address string) ([]AddressKladr, error) {
	u := c.baseURL + "/parse?address=" + encodeQueryValue(address)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geo /parse: unexpected status %d", resp.StatusCode)
	}
	var out []AddressKladr
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("geo /parse: decode: %w", err)
	}
	return out, nil
}
