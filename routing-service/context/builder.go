package context

import (
	"math"
)

type ContextService struct {
	NearestEdge func(lat, lng float64) string
	NodeLabel   func(lat, lng float64) string

	TrafficFetcher TrafficFetcher
	WeatherFetcher WeatherFetcher
	NewsSource     NewsSource
	NewsAnalyzer   NewsAnalyzer

	RegionRadiusKm float64
}

func NewContextService(nearestEdge func(lat, lng float64) string, nodeLabel func(lat, lng float64) string) *ContextService {
	return &ContextService{
		NearestEdge:    nearestEdge,
		NodeLabel:      nodeLabel,
		TrafficFetcher: NewTomTomTrafficFetcher(),
		WeatherFetcher: NewOpenMeteoWeatherFetcher(),
		NewsSource:     NewGoogleNewsSource(),
		NewsAnalyzer:   NewOpenAINewsAnalyzer(),
		RegionRadiusKm: 25,
	}
}

func (c *ContextService) Build(lat, lng float64) Context {
	ctx := BuildContext()
	if c == nil {
		return ctx
	}

	nearestEdge := c.NearestEdge
	if nearestEdge == nil {
		nearestEdge = func(lat, lng float64) string { return "" }
	}

	if c.TrafficFetcher != nil {
		if bounds, ok := boundsAround(lat, lng, c.RegionRadiusKm); ok {
			if signals, err := c.TrafficFetcher.Fetch(bounds); err == nil {
				for _, signal := range signals {
					edgeID := nearestEdge(signal.Latitude, signal.Longitude)
					if edgeID == "" {
						continue
					}
					applyFactor(&ctx, edgeID, EdgeContext{TrafficFactor: signal.Factor})
				}
			}
		}
	}

	if c.WeatherFetcher != nil {
		if factor, err := c.WeatherFetcher.Fetch(lat, lng); err == nil {
			edgeID := nearestEdge(lat, lng)
			if edgeID != "" {
				applyFactor(&ctx, edgeID, EdgeContext{WeatherFactor: factor})
			}
		}
	}

	if c.NewsSource != nil && c.NewsAnalyzer != nil {
		query := ""
		if c.NodeLabel != nil {
			query = c.NodeLabel(lat, lng)
		}
		if headlines, err := c.NewsSource.Fetch(query); err == nil && len(headlines) > 0 {
			if factor, err := c.NewsAnalyzer.Analyze(headlines); err == nil {
				edgeID := nearestEdge(lat, lng)
				if edgeID != "" {
					applyFactor(&ctx, edgeID, EdgeContext{NewsFactor: factor})
				}
			}
		}
	}

	return ctx
}

func applyFactor(ctx *Context, edgeID string, delta EdgeContext) {
	existing := ctx.EdgeFactors[edgeID]

	if delta.TrafficFactor > existing.TrafficFactor {
		existing.TrafficFactor = delta.TrafficFactor
	}
	if delta.WeatherFactor > existing.WeatherFactor {
		existing.WeatherFactor = delta.WeatherFactor
	}
	if delta.NewsFactor > existing.NewsFactor {
		existing.NewsFactor = delta.NewsFactor
	}

	ctx.EdgeFactors[edgeID] = existing
}

func boundsAround(lat, lng, radiusKm float64) (BoundingBox, bool) {
	if radiusKm <= 0 {
		return BoundingBox{}, false
	}

	latDelta := radiusKm / 111.0
	lonScale := math.Cos(lat * math.Pi / 180.0)
	if lonScale == 0 {
		lonScale = 1e-6
	}
	lonDelta := radiusKm / (111.0 * lonScale)

	return BoundingBox{
		MinLon: lng - lonDelta,
		MinLat: lat - latDelta,
		MaxLon: lng + lonDelta,
		MaxLat: lat + latDelta,
	}, true
}
