package geoservice

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	Provider string
	APIKey   string
	BaseURL  string
	Client   *http.Client
}

func ConfigFromEnv() (Config, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("GEOCODER_PROVIDER")))
	baseURL := strings.TrimSpace(os.Getenv("GEOCODER_BASE_URL"))

	if provider == "" {
		switch {
		case strings.TrimSpace(os.Getenv("TOMTOM_API_KEY")) != "" || strings.TrimSpace(os.Getenv("GEOCODER_API_KEY")) != "":
			provider = "tomtom"
		case strings.TrimSpace(os.Getenv("OPENCAGE_API_KEY")) != "":
			provider = "opencage"
		case strings.TrimSpace(os.Getenv("GOOGLE_MAPS_API_KEY")) != "" || strings.TrimSpace(os.Getenv("GOOGLE_API_KEY")) != "":
			provider = "google"
		default:
			provider = "tomtom"
		}
	}

	var apiKey string

	switch provider {
	case "tomtom":
		apiKey = firstNonEmpty(os.Getenv("TOMTOM_API_KEY"), os.Getenv("GEOCODER_API_KEY"))
		if baseURL == "" {
			baseURL = "https://api.tomtom.com"
		}
	case "opencage":
		apiKey = firstNonEmpty(os.Getenv("OPENCAGE_API_KEY"), os.Getenv("GEOCODER_API_KEY"))
		if baseURL == "" {
			baseURL = "https://api.opencagedata.com"
		}
	case "google":
		apiKey = firstNonEmpty(os.Getenv("GOOGLE_MAPS_API_KEY"), os.Getenv("GOOGLE_API_KEY"), os.Getenv("GEOCODER_API_KEY"))
		if baseURL == "" {
			baseURL = "https://maps.googleapis.com"
		}
	default:
		return Config{}, fmt.Errorf("unsupported geocoding provider %q", provider)
	}

	if apiKey == "" {
		return Config{}, fmt.Errorf("geocoding API key is not configured for provider %q", provider)
	}

	return Config{
		Provider: provider,
		APIKey:   apiKey,
		BaseURL:  baseURL,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
