package context

import (
	"math"
	"testing"
	"time"
)

type trafficFetcherStub struct {
	bounds BoundingBox
}

type weatherFetcherStub struct {
	points [][2]float64
}

func (f *trafficFetcherStub) Fetch(bounds BoundingBox, at time.Time) ([]TrafficSignal, error) {
	f.bounds = bounds
	return []TrafficSignal{
		{Latitude: 0, Longitude: 0, Factor: 1.5},
	}, nil
}

func (f *weatherFetcherStub) Fetch(lat, lng float64, at time.Time) (float64, error) {
	f.points = append(f.points, [2]float64{lat, lng})
	return 1.3, nil
}

type newsSourceStub struct {
	query string
}

func (s *newsSourceStub) Fetch(query string) ([]string, error) {
	s.query = query
	return []string{"route disruption reported"}, nil
}

type newsAnalyzerStub struct{}

func (newsAnalyzerStub) Analyze(headlines []string) (float64, error) {
	return 1.8, nil
}

func TestBuildForRouteUsesRouteBoundsAndCenterPoint(t *testing.T) {
	traffic := &trafficFetcherStub{}
	weather := &weatherFetcherStub{}
	news := &newsSourceStub{}

	svc := NewContextService(func(lat, lng float64) string {
		return "edge-center"
	}, func(lat, lng float64) string {
		return "route-center"
	})
	svc.RouteMarginKm = 20
	svc.NearbyEdges = func(lat, lng, radiusKm float64) []EdgeDistance {
		return []EdgeDistance{
			{EdgeID: "edge-1", DistanceKm: 0.1},
			{EdgeID: "edge-2", DistanceKm: 0.2},
		}
	}
	svc.TrafficFetcher = traffic
	svc.WeatherFetcher = weather
	svc.NewsSource = news
	svc.NewsAnalyzer = newsAnalyzerStub{}

	ctx := svc.BuildForRoute(0, 0, 0, 1, time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC))

	wantDelta := 20.0 / 111.0
	if diff := math.Abs(traffic.bounds.MinLat - (-wantDelta)); diff > 0.01 {
		t.Fatalf("unexpected min lat: got %v want approx %v", traffic.bounds.MinLat, -wantDelta)
	}
	if diff := math.Abs(traffic.bounds.MaxLat - wantDelta); diff > 0.01 {
		t.Fatalf("unexpected max lat: got %v want approx %v", traffic.bounds.MaxLat, wantDelta)
	}
	if diff := math.Abs(traffic.bounds.MinLon - (-wantDelta)); diff > 0.01 {
		t.Fatalf("unexpected min lon: got %v want approx %v", traffic.bounds.MinLon, -wantDelta)
	}
	if diff := math.Abs(traffic.bounds.MaxLon - (1 + wantDelta)); diff > 0.01 {
		t.Fatalf("unexpected max lon: got %v want approx %v", traffic.bounds.MaxLon, 1+wantDelta)
	}

	if len(weather.points) != 3 {
		t.Fatalf("expected 3 weather lookups, got %d", len(weather.points))
	}
	if news.query != "route-center" {
		t.Fatalf("unexpected news query: got %q", news.query)
	}

	factors, ok := ctx.EdgeFactors["edge-1"]
	if !ok {
		t.Fatal("expected edge-1 factors to be populated")
	}
	if factors.TrafficFactor <= 1.0 || factors.TrafficFactor >= 1.5 {
		t.Fatalf("unexpected traffic factor: %v", factors.TrafficFactor)
	}
	if factors.WeatherFactor <= 1.0 {
		t.Fatalf("expected weather factor on edge-1, got %v", factors.WeatherFactor)
	}
	if factors.NewsFactor <= 1.0 {
		t.Fatalf("expected news factor on edge-1, got %v", factors.NewsFactor)
	}

	factors, ok = ctx.EdgeFactors["edge-2"]
	if !ok {
		t.Fatal("expected edge-2 factors to be populated")
	}
	if factors.TrafficFactor <= 1.0 || factors.TrafficFactor >= 1.5 {
		t.Fatalf("unexpected edge-2 traffic factor: %v", factors.TrafficFactor)
	}
	if factors.WeatherFactor <= 1.0 {
		t.Fatalf("expected weather factor on edge-2, got %v", factors.WeatherFactor)
	}
	if factors.NewsFactor <= 1.0 {
		t.Fatalf("expected news factor on edge-2, got %v", factors.NewsFactor)
	}

	if _, ok := ctx.EdgeArrivalTimes["edge-1"]; !ok {
		t.Fatal("expected edge-1 arrival time to be populated")
	}
	if _, ok := ctx.EdgeArrivalTimes["edge-2"]; !ok {
		t.Fatal("expected edge-2 arrival time to be populated")
	}
}

func TestNewContextServiceUsesRouteMarginEnv(t *testing.T) {
	t.Setenv("CONTEXT_ROUTE_MARGIN_KM", "22")

	svc := NewContextService(nil, nil)
	if svc.RouteMarginKm != 22 {
		t.Fatalf("expected route margin 22, got %v", svc.RouteMarginKm)
	}
}
