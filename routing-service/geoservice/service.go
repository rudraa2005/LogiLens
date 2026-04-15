package geoservice

import (
	"context"
	"errors"
	"strings"
	"sync"

	"golang.org/x/sync/singleflight"
)

type LatLng struct {
	Latitude  float64
	Longitude float64
}

type GeoService struct {
	provider Provider

	cacheMu sync.RWMutex
	cache   map[string]LatLng

	lookupGroup singleflight.Group
}

func New(provider Provider) *GeoService {
	return &GeoService{
		provider: provider,
		cache:    make(map[string]LatLng),
	}
}

func NewFromEnv() (*GeoService, error) {
	cfg, err := ConfigFromEnv()
	if err != nil {
		return nil, err
	}

	provider, err := NewProvider(cfg)
	if err != nil {
		return nil, err
	}

	return New(provider), nil
}

func (g *GeoService) Geocode(place string) (float64, float64, error) {
	if g == nil {
		return 0, 0, errors.New("geoservice is nil")
	}

	normalized := normalizePlace(place)
	if normalized == "" {
		return 0, 0, errors.New("place cannot be empty")
	}

	if latLng, ok := g.getCached(normalized); ok {
		return latLng.Latitude, latLng.Longitude, nil
	}

	if g.provider == nil {
		return 0, 0, errors.New("geocoding provider is not configured")
	}

	value, err, _ := g.lookupGroup.Do(normalized, func() (any, error) {
		if latLng, ok := g.getCached(normalized); ok {
			return latLng, nil
		}

		latLng, err := g.provider.Geocode(context.Background(), normalized)
		if err != nil {
			return LatLng{}, err
		}

		g.setCached(normalized, latLng)
		return latLng, nil
	})
	if err != nil {
		return 0, 0, err
	}

	latLng, ok := value.(LatLng)
	if !ok {
		return 0, 0, errors.New("invalid geocoding result type")
	}

	return latLng.Latitude, latLng.Longitude, nil
}

func normalizePlace(place string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(place))), " ")
}

func (g *GeoService) getCached(key string) (LatLng, bool) {
	g.cacheMu.RLock()
	defer g.cacheMu.RUnlock()

	value, ok := g.cache[key]
	return value, ok
}

func (g *GeoService) setCached(key string, value LatLng) {
	g.cacheMu.Lock()
	defer g.cacheMu.Unlock()

	g.cache[key] = value
}
