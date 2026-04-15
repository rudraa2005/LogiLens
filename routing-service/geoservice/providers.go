package geoservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Provider interface {
	Geocode(ctx context.Context, place string) (LatLng, error)
}

type TomTomProvider struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

type OpenCageProvider struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

type GoogleProvider struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

func NewProvider(cfg Config) (Provider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "", "tomtom":
		return &TomTomProvider{
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey,
			Client:  cfg.Client,
		}, nil
	case "opencage":
		return &OpenCageProvider{
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey,
			Client:  cfg.Client,
		}, nil
	case "google":
		return &GoogleProvider{
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey,
			Client:  cfg.Client,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported geocoding provider %q", cfg.Provider)
	}
}

func (p *TomTomProvider) Geocode(ctx context.Context, place string) (LatLng, error) {
	baseURL := geocodingBaseURL(p.BaseURL, "https://api.tomtom.com")
	client := geocodingClient(p.Client)

	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return LatLng{}, fmt.Errorf("invalid tomtom geocoding endpoint: %w", err)
	}

	escapedPlace := url.PathEscape(place)
	endpoint.Path = fmt.Sprintf("/search/2/geocode/%s.json", place)
	endpoint.RawPath = fmt.Sprintf("/search/2/geocode/%s.json", escapedPlace)
	query := endpoint.Query()
	query.Set("key", p.APIKey)
	query.Set("limit", "1")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return LatLng{}, err
	}

	resp, err := client.Do(req)
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
			return LatLng{}, fmt.Errorf("tomtom geocoding failed: %s", body.ErrorText)
		}
		return LatLng{}, fmt.Errorf("tomtom geocoding failed: http %d", resp.StatusCode)
	}

	if body.ErrorText != "" {
		return LatLng{}, fmt.Errorf("tomtom geocoding failed: %s", body.ErrorText)
	}

	if len(body.Results) == 0 {
		return LatLng{}, fmt.Errorf("tomtom geocoding returned no results for %q", place)
	}

	position := body.Results[0].Position
	return LatLng{Latitude: position.Lat, Longitude: position.Lon}, nil
}

func (p *OpenCageProvider) Geocode(ctx context.Context, place string) (LatLng, error) {
	baseURL := geocodingBaseURL(p.BaseURL, "https://api.opencagedata.com")
	client := geocodingClient(p.Client)

	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return LatLng{}, fmt.Errorf("invalid opencage geocoding endpoint: %w", err)
	}

	endpoint.Path = "/geocode/v1/json"
	query := endpoint.Query()
	query.Set("q", place)
	query.Set("key", p.APIKey)
	query.Set("limit", "1")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return LatLng{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return LatLng{}, err
	}
	defer resp.Body.Close()

	var body openCageResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return LatLng{}, err
	}

	if resp.StatusCode != http.StatusOK {
		if body.Status.Message != "" {
			return LatLng{}, fmt.Errorf("opencage geocoding failed: %s", body.Status.Message)
		}
		return LatLng{}, fmt.Errorf("opencage geocoding failed: http %d", resp.StatusCode)
	}

	if len(body.Results) == 0 {
		return LatLng{}, fmt.Errorf("opencage geocoding returned no results for %q", place)
	}

	geometry := body.Results[0].Geometry
	return LatLng{Latitude: geometry.Lat, Longitude: geometry.Lng}, nil
}

func (p *GoogleProvider) Geocode(ctx context.Context, place string) (LatLng, error) {
	baseURL := geocodingBaseURL(p.BaseURL, "https://maps.googleapis.com")
	client := geocodingClient(p.Client)

	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return LatLng{}, fmt.Errorf("invalid google geocoding endpoint: %w", err)
	}

	endpoint.Path = "/maps/api/geocode/json"
	query := endpoint.Query()
	query.Set("address", place)
	query.Set("key", p.APIKey)
	query.Set("limit", "1")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return LatLng{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return LatLng{}, err
	}
	defer resp.Body.Close()

	var body googleResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return LatLng{}, err
	}

	if resp.StatusCode != http.StatusOK {
		if body.ErrorMessage != "" {
			return LatLng{}, fmt.Errorf("google geocoding failed: %s", body.ErrorMessage)
		}
		return LatLng{}, fmt.Errorf("google geocoding failed: http %d", resp.StatusCode)
	}

	if strings.TrimSpace(body.Status) != "" && !strings.EqualFold(body.Status, "OK") {
		if body.ErrorMessage != "" {
			return LatLng{}, fmt.Errorf("google geocoding failed: %s", body.ErrorMessage)
		}
		return LatLng{}, fmt.Errorf("google geocoding failed: %s", body.Status)
	}

	if len(body.Results) == 0 {
		return LatLng{}, fmt.Errorf("google geocoding returned no results for %q", place)
	}

	location := body.Results[0].Geometry.Location
	return LatLng{Latitude: location.Lat, Longitude: location.Lng}, nil
}

func geocodingBaseURL(baseURL, fallback string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return fallback
	}
	return baseURL
}

func geocodingClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{}
}

type tomTomResponse struct {
	ErrorText string `json:"errorText"`
	Results   []struct {
		Position struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"position"`
	} `json:"results"`
}

type openCageResponse struct {
	Status struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"status"`
	Results []struct {
		Geometry struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"geometry"`
	} `json:"results"`
}

type googleResponse struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	Results      []struct {
		Geometry struct {
			Location struct {
				Lat float64 `json:"lat"`
				Lng float64 `json:"lng"`
			} `json:"location"`
		} `json:"geometry"`
	} `json:"results"`
}
