package context

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type WeatherFetcher interface {
	Fetch(lat, lng float64, at time.Time) (float64, error)
}

type OpenMeteoWeatherFetcher struct {
	BaseURL string
	Client  *http.Client
}

func NewOpenMeteoWeatherFetcher() *OpenMeteoWeatherFetcher {
	return &OpenMeteoWeatherFetcher{
		BaseURL: "https://api.open-meteo.com",
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (f *OpenMeteoWeatherFetcher) Fetch(lat, lng float64, at time.Time) (float64, error) {
	baseURL := f.BaseURL
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://api.open-meteo.com"
	}

	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return 1.0, fmt.Errorf("invalid weather endpoint: %w", err)
	}

	endpoint.Path = "/v1/forecast"
	query := endpoint.Query()
	query.Set("latitude", strconv.FormatFloat(lat, 'f', -1, 64))
	query.Set("longitude", strconv.FormatFloat(lng, 'f', -1, 64))
	query.Set("hourly", "weather_code,precipitation,wind_speed_10m")
	query.Set("forecast_days", "2")
	query.Set("timezone", "UTC")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return 1.0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return 1.0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 1.0, fmt.Errorf("weather lookup failed: http %d", resp.StatusCode)
	}

	var body struct {
		Hourly struct {
			Time          []string  `json:"time"`
			WeatherCode   []int     `json:"weather_code"`
			Precipitation []float64 `json:"precipitation"`
			WindSpeed10M  []float64 `json:"wind_speed_10m"`
		} `json:"hourly"`
		Current struct {
			WeatherCode   int     `json:"weather_code"`
			Precipitation float64 `json:"precipitation"`
			WindSpeed10M  float64 `json:"wind_speed_10m"`
		} `json:"current"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 1.0, err
	}

	if factor, ok := forecastFactorAt(body.Hourly.Time, body.Hourly.WeatherCode, body.Hourly.Precipitation, body.Hourly.WindSpeed10M, at); ok {
		return factor, nil
	}

	return weatherFactorForCode(body.Current.WeatherCode, body.Current.Precipitation, body.Current.WindSpeed10M), nil
}

func forecastFactorAt(times []string, codes []int, precipitation []float64, wind []float64, at time.Time) (float64, bool) {
	if len(times) == 0 {
		return 1.0, false
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}

	target := at.UTC().Truncate(time.Hour)
	bestIndex := -1
	bestDelta := 48 * time.Hour

	for i, raw := range times {
		parsed, err := time.Parse("2006-01-02T15:04", raw)
		if err != nil {
			continue
		}
		delta := parsed.Sub(target)
		if delta < 0 {
			delta = -delta
		}
		if delta < bestDelta {
			bestDelta = delta
			bestIndex = i
		}
	}

	if bestIndex < 0 || bestIndex >= len(codes) {
		return 1.0, false
	}

	precip := 0.0
	if bestIndex < len(precipitation) {
		precip = precipitation[bestIndex]
	}
	windSpeed := 0.0
	if bestIndex < len(wind) {
		windSpeed = wind[bestIndex]
	}

	return weatherFactorForCode(codes[bestIndex], precip, windSpeed), true
}

func weatherFactorForCode(code int, precipitation, windSpeed float64) float64 {
	switch {
	case code == 0 || code == 1:
		return 1.0
	case code == 2 || code == 3:
		return 1.1
	case code == 45 || code == 48:
		return 1.2
	case code == 51 || code == 53 || code == 55 || code == 56 || code == 57:
		return 1.2
	case code == 61 || code == 63 || code == 80 || code == 81:
		return 1.3
	case code == 65 || code == 82 || precipitation >= 15 || windSpeed >= 50:
		return 1.5
	case code == 71 || code == 73 || code == 75 || code == 77 || code == 85 || code == 86:
		return 1.5
	case code == 95 || code == 96 || code == 99:
		return 2.0
	default:
		return 1.0
	}
}
