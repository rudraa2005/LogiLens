package context

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rudraa2005/LogiLens/routing-service/models"
)

type BoundingBox struct {
	MinLon float64
	MinLat float64
	MaxLon float64
	MaxLat float64
}

type TrafficSignal struct {
	Latitude  float64
	Longitude float64
	Factor    float64
}

type TrafficFetcher interface {
	Fetch(bounds BoundingBox) ([]TrafficSignal, error)
}

const (
	defaultTomTomTrafficBaseURL = "https://api.tomtom.com"
	defaultTomTomTrafficAPIKey  = "yVyanidds6k8iaGvtgfgDaUp2n2791af"
)

var (
	trafficAPIKey     = func() string { return firstNonEmpty(os.Getenv("TOMTOM_API_KEY"), defaultTomTomTrafficAPIKey) }()
	trafficBaseURL    = defaultTomTomTrafficBaseURL
	trafficHTTPClient = &http.Client{Timeout: 10 * time.Second}
)

type TomTomTrafficFetcher struct{}

func NewTomTomTrafficFetcher() *TomTomTrafficFetcher {
	return &TomTomTrafficFetcher{}
}

type trafficIncidentResponse struct {
	Incidents []trafficIncident `json:"incidents"`
}

type trafficIncident struct {
	Geometry struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"geometry"`
	Properties struct {
		IconCategory     int `json:"iconCategory"`
		MagnitudeOfDelay int `json:"magnitudeOfDelay"`
	} `json:"properties"`
}

func (f *TomTomTrafficFetcher) Fetch(bounds BoundingBox) ([]TrafficSignal, error) {
	incidents, err := fetchTrafficIncidents(bounds)
	if err != nil {
		return nil, err
	}

	var signals []TrafficSignal
	for _, incident := range incidents {
		factor := trafficFactorForIncident(incident.Properties.IconCategory, incident.Properties.MagnitudeOfDelay)
		points, err := extractTrafficPoints(incident.Geometry.Coordinates)
		if err != nil {
			continue
		}

		for _, point := range points {
			signals = append(signals, TrafficSignal{
				Latitude:  point.Latitude,
				Longitude: point.Longitude,
				Factor:    factor,
			})
		}
	}

	return signals, nil
}

func fetchTrafficIncidents(bounds BoundingBox) ([]trafficIncident, error) {
	if trafficAPIKey == "" {
		return nil, errors.New("TOMTOM_API_KEY is not configured")
	}

	endpoint, err := url.Parse(trafficBaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid traffic endpoint: %w", err)
	}

	endpoint.Path = "/traffic/services/5/incidentDetails"
	query := endpoint.Query()
	query.Set("key", trafficAPIKey)
	query.Set("bbox", bounds.String())
	query.Set("fields", "{incidents{geometry{type,coordinates},properties{iconCategory,magnitudeOfDelay}}}")
	query.Set("language", "en-GB")
	query.Set("timeValidityFilter", "present")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := trafficHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("traffic lookup failed: http %d", resp.StatusCode)
	}

	var body trafficIncidentResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}

	return body.Incidents, nil
}

func (b BoundingBox) String() string {
	return strings.Join([]string{
		formatFloat(b.MinLon),
		formatFloat(b.MinLat),
		formatFloat(b.MaxLon),
		formatFloat(b.MaxLat),
	}, ",")
}

func trafficFactorForIncident(iconCategory, magnitudeOfDelay int) float64 {
	switch {
	case magnitudeOfDelay >= 3 || iconCategory == 8:
		return 2.0
	case magnitudeOfDelay == 2 || iconCategory == 6 || iconCategory == 7:
		return 1.5
	case magnitudeOfDelay == 1 || iconCategory == 5:
		return 1.2
	default:
		return 1.0
	}
}

func extractTrafficPoints(raw json.RawMessage) ([]models.LatLng, error) {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, err
	}

	var points []models.LatLng
	collectTrafficPoints(decoded, &points)
	return points, nil
}

func collectTrafficPoints(value any, points *[]models.LatLng) {
	switch v := value.(type) {
	case []any:
		if len(v) == 2 {
			lon, okLon := toFloat64(v[0])
			lat, okLat := toFloat64(v[1])
			if okLon && okLat {
				*points = append(*points, models.LatLng{
					Latitude:  lat,
					Longitude: lon,
				})
				return
			}
		}

		for _, item := range v {
			collectTrafficPoints(item, points)
		}
	}
}

func toFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
