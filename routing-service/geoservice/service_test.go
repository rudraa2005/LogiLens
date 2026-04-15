package geoservice

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type fakeProvider struct {
	mu     sync.Mutex
	calls  []string
	result LatLng
	err    error
}

func (f *fakeProvider) Geocode(ctx context.Context, place string) (LatLng, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, place)
	if f.err != nil {
		return LatLng{}, f.err
	}

	return f.result, nil
}

func TestGeoServiceGeocodeCachesNormalizedInputs(t *testing.T) {
	provider := &fakeProvider{
		result: LatLng{Latitude: 12.34, Longitude: 56.78},
	}
	svc := New(provider)

	lat, lng, err := svc.Geocode("  New   York ")
	if err != nil {
		t.Fatalf("Geocode returned error: %v", err)
	}
	if lat != 12.34 || lng != 56.78 {
		t.Fatalf("unexpected coordinates: got (%v,%v)", lat, lng)
	}

	lat, lng, err = svc.Geocode("new york")
	if err != nil {
		t.Fatalf("Geocode returned error on cache hit: %v", err)
	}
	if lat != 12.34 || lng != 56.78 {
		t.Fatalf("unexpected cached coordinates: got (%v,%v)", lat, lng)
	}

	provider.mu.Lock()
	defer provider.mu.Unlock()

	if len(provider.calls) != 1 {
		t.Fatalf("expected provider to be called once, got %d calls: %#v", len(provider.calls), provider.calls)
	}
	if provider.calls[0] != "new york" {
		t.Fatalf("expected normalized lookup key, got %q", provider.calls[0])
	}
}

func TestGeoServiceGeocodeRejectsEmptyPlace(t *testing.T) {
	svc := New(&fakeProvider{result: LatLng{Latitude: 1, Longitude: 2}})

	_, _, err := svc.Geocode("   ")
	if err == nil {
		t.Fatal("expected error for empty place")
	}
}

func TestGeoServiceGeocodePropagatesProviderErrors(t *testing.T) {
	svc := New(&fakeProvider{err: errors.New("boom")})

	_, _, err := svc.Geocode("Berlin")
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected provider error to be returned, got %v", err)
	}
}

func TestTomTomProviderGeocode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.EscapedPath(); got != "/search/2/geocode/new%20york.json" {
			t.Fatalf("unexpected path: got %q", got)
		}
		if got := r.URL.Query().Get("key"); got != "secret" {
			t.Fatalf("unexpected key query param: got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"results":[{"position":{"lat":40.7128,"lon":-74.0060}}]}`)
	}))
	defer server.Close()

	provider := &TomTomProvider{
		BaseURL: server.URL,
		APIKey:  "secret",
		Client:  server.Client(),
	}

	latLng, err := provider.Geocode(context.Background(), "new york")
	if err != nil {
		t.Fatalf("Geocode returned error: %v", err)
	}
	if latLng.Latitude != 40.7128 || latLng.Longitude != -74.006 {
		t.Fatalf("unexpected coordinates: %#v", latLng)
	}
}
