package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_Find(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/parse" || r.URL.Query().Get("address") != "Тирасполь Ленина" {
			t.Errorf("unexpected request: %s %s", r.URL.Path, r.URL.RawQuery)
		}
		// Spring's UriComponentsBuilder encodes space as %20, not the
		// application/x-www-form-urlencoded '+'. A geo backend that
		// doesn't form-decode must still see %20.
		if strings.Contains(r.URL.RawQuery, "+") || !strings.Contains(r.URL.RawQuery, "%20") {
			t.Errorf("expected %%20-encoded space in raw query, got: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"country":"MD","full_address":"г. Тирасполь, ул. Ленина",
			"kladr":"123-01.001-02.002-00.000-04.004",
			"region":{"kladr":"123-01.001-00.000-00.000-00.000","name":"Слободзейский","type":"р-н"},
			"city":{"kladr":"123-01.001-02.002-00.000-00.000","name":"Тирасполь","type":"г."},
			"street":{"kladr":"123-01.001-02.002-00.000-04.004","name":"Ленина","type":"ул.","name_with_type":"ул. Ленина","type_full":"улица"}}]`))
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL).Find(context.Background(), "Тирасполь Ленина")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Street.NameWithType != "ул. Ленина" ||
		got[0].FullAddress != "г. Тирасполь, ул. Ленина" {
		t.Fatalf("%+v", got)
	}
}

func TestClient_Find_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	if _, err := NewClient(srv.URL).Find(context.Background(), "x"); err == nil {
		t.Fatal("expected error on 500")
	}
}
