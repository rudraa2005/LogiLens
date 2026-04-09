package geocoder

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type LatLng struct {
	Latitude  float64
	Longitude float64
}

const (
	defaultTomTomBaseURL = "https://api.tomtom.com"
	defaultTomTomAPIKey  = "yVyanidds6k8iaGvtgfgDaUp2n2791af"
)

var (
	cacheMu    sync.RWMutex
	cache      = make(map[string]LatLng)
	apiKey     = defaultTomTomAPIKey
	baseURL    = defaultTomTomBaseURL
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
)

type tomTomResponse struct {
	Summary struct {
		Query      string `json:"query"`
		QueryType  string `json:"queryType"`
		QueryTime  int    `json:"queryTime"`
		NumResults int    `json:"numResults"`
		Offset     int    `json:"offset"`
		Total      int    `json:"totalResults"`
		FuzzyLevel int    `json:"fuzzyLevel"`
	} `json:"summary"`
	Results []struct {
		Position struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"position"`
	} `json:"results"`
	ErrorText     string `json:"errorText"`
	DetailedError struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Target  string `json:"target"`
	} `json:"detailedError"`
	HttpStatusCode string `json:"httpStatusCode"`
}

func Geocode(place string) (float64, float64, error) {
	normalized := normalizePlace(place)
	if normalized == "" {
		return 0, 0, errors.New("place cannot be empty")
	}

	if latLng, ok := getCached(normalized); ok {
		return latLng.Latitude, latLng.Longitude, nil
	}

	if strings.TrimSpace(apiKey) == "" {
		return 0, 0, errors.New("TOMTOM_API_KEY is not configured")
	}

	latLng, err := fetchGeocode(normalized)
	if err != nil {
		return 0, 0, err
	}

	setCache(normalized, latLng)
	return latLng.Latitude, latLng.Longitude, nil
}

func normalizePlace(place string) string {
	return strings.ToLower(strings.TrimSpace(place))
}

func getCached(key string) (LatLng, bool) {
	cacheMu.RLock()
	defer cacheMu.RUnlock()

	value, ok := cache[key]
	return value, ok
}

func setCache(key string, value LatLng) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	cache[key] = value
}

func fetchGeocode(place string) (LatLng, error) {
	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return LatLng{}, fmt.Errorf("invalid geocoding endpoint: %w", err)
	}

	endpoint.Path = fmt.Sprintf("/search/2/geocode/%s.json", url.PathEscape(place))
	query := endpoint.Query()
	query.Set("key", apiKey)
	query.Set("limit", "1")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return LatLng{}, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return LatLng{}, err
	}
	defer resp.Body.Close()

	var body tomTomResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return LatLng{}, err
	}

	if resp.StatusCode != http.StatusOK {
		if body.ErrorText != "" {
			return LatLng{}, fmt.Errorf("geocoding failed: %s", body.ErrorText)
		}
		return LatLng{}, fmt.Errorf("geocoding failed: http %d", resp.StatusCode)
	}

	if body.ErrorText != "" {
		return LatLng{}, fmt.Errorf("geocoding failed: %s", body.ErrorText)
	}

	if len(body.Results) == 0 {
		return LatLng{}, errors.New("no geocoding results found")
	}

	result := body.Results[0].Position
	return LatLng{
		Latitude:  result.Lat,
		Longitude: result.Lon,
	}, nil
}
