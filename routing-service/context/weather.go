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
	Fetch(lat, lng float64) (float64, error)
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

func (f *OpenMeteoWeatherFetcher) Fetch(lat, lng float64) (float64, error) {
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
	query.Set("current", "weather_code,precipitation,wind_speed_10m")
	query.Set("timezone", "auto")
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
		Current struct {
			WeatherCode   int     `json:"weather_code"`
			Precipitation float64 `json:"precipitation"`
			WindSpeed10M  float64 `json:"wind_speed_10m"`
		} `json:"current"`
		CurrentWeather struct {
			WeatherCode   int     `json:"weathercode"`
			Precipitation float64 `json:"precipitation"`
			WindSpeed10M  float64 `json:"windspeed"`
		} `json:"current_weather"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 1.0, err
	}

	code := body.Current.WeatherCode
	if code == 0 && body.CurrentWeather.WeatherCode != 0 {
		code = body.CurrentWeather.WeatherCode
	}

	return weatherFactorForCode(code, body.Current.Precipitation, body.Current.WindSpeed10M), nil
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
