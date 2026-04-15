package context

import (
	"math"
	"os"
	"strconv"
	"strings"
)

type ContextConfig struct {
	RouteMarginKm float64
}

type ContextService struct {
	NearestEdge func(lat, lng float64) string
	NearbyEdges func(lat, lng float64) []string
	NodeLabel   func(lat, lng float64) string

	TrafficFetcher TrafficFetcher
	WeatherFetcher WeatherFetcher
	NewsSource     NewsSource
	NewsAnalyzer   NewsAnalyzer

	RouteMarginKm float64
}

func NewContextService(nearestEdge func(lat, lng float64) string, nodeLabel func(lat, lng float64) string) *ContextService {
	return NewContextServiceWithConfig(nearestEdge, nodeLabel, contextConfigFromEnv())
}

func NewContextServiceWithConfig(nearestEdge func(lat, lng float64) string, nodeLabel func(lat, lng float64) string, cfg ContextConfig) *ContextService {
	marginKm := clampMarginKm(cfg.RouteMarginKm)
	if marginKm == 0 {
		marginKm = 15
	}

	return &ContextService{
		NearestEdge:    nearestEdge,
		NodeLabel:      nodeLabel,
		TrafficFetcher: NewTomTomTrafficFetcher(),
		WeatherFetcher: NewOpenMeteoWeatherFetcher(),
		NewsSource:     NewGoogleNewsSource(),
		NewsAnalyzer:   NewOpenAINewsAnalyzer(),
		RouteMarginKm:  marginKm,
	}
}

func contextConfigFromEnv() ContextConfig {
	marginKm := 15.0
	if raw := firstNonEmpty(os.Getenv("CONTEXT_ROUTE_MARGIN_KM"), os.Getenv("ROUTE_MARGIN_KM")); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			marginKm = parsed
		}
	}

	return ContextConfig{
		RouteMarginKm: marginKm,
	}
}

func (c *ContextService) BuildForRoute(srcLat, srcLng, dstLat, dstLng float64) Context {
	ctx := BuildContext()
	if c == nil {
		return ctx
	}

	nearestEdge := c.NearestEdge
	if nearestEdge == nil {
		nearestEdge = func(lat, lng float64) string { return "" }
	}

	bounds, ok := routeBoundsAround(srcLat, srcLng, dstLat, dstLng, c.RouteMarginKm)
	if !ok {
		return ctx
	}

	routeLat := (bounds.MinLat + bounds.MaxLat) / 2
	routeLng := (bounds.MinLon + bounds.MaxLon) / 2

	if c.TrafficFetcher != nil {
		if signals, err := c.TrafficFetcher.Fetch(bounds); err == nil {
			for _, signal := range signals {
				edgeIDs := c.edgeIDsForSignal(signal.Latitude, signal.Longitude, nearestEdge)
				if len(edgeIDs) == 0 {
					continue
				}
				for _, edgeID := range edgeIDs {
					applyFactor(&ctx, edgeID, EdgeContext{TrafficFactor: signal.Factor})
				}
			}
		}
	}

	if c.WeatherFetcher != nil {
		if factor, err := c.WeatherFetcher.Fetch(routeLat, routeLng); err == nil {
			edgeID := nearestEdge(routeLat, routeLng)
			if edgeID != "" {
				applyFactor(&ctx, edgeID, EdgeContext{WeatherFactor: factor})
			}
		}
	}

	if c.NewsSource != nil && c.NewsAnalyzer != nil {
		query := ""
		if c.NodeLabel != nil {
			query = c.NodeLabel(routeLat, routeLng)
		}
		if headlines, err := c.NewsSource.Fetch(query); err == nil && len(headlines) > 0 {
			if factor, err := c.NewsAnalyzer.Analyze(headlines); err == nil {
				edgeID := nearestEdge(routeLat, routeLng)
				if edgeID != "" {
					applyFactor(&ctx, edgeID, EdgeContext{NewsFactor: factor})
				}
			}
		}
	}

	return ctx
}

func (c *ContextService) edgeIDsForSignal(lat, lng float64, fallback func(lat, lng float64) string) []string {
	edgeIDs := []string{}
	seen := make(map[string]struct{})

	if c != nil && c.NearbyEdges != nil {
		for _, edgeID := range c.NearbyEdges(lat, lng) {
			edgeID = strings.TrimSpace(edgeID)
			if edgeID == "" {
				continue
			}
			if _, ok := seen[edgeID]; ok {
				continue
			}
			seen[edgeID] = struct{}{}
			edgeIDs = append(edgeIDs, edgeID)
		}
	}

	if len(edgeIDs) > 0 {
		return edgeIDs
	}

	if fallback != nil {
		if edgeID := strings.TrimSpace(fallback(lat, lng)); edgeID != "" {
			return []string{edgeID}
		}
	}

	return nil
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

func routeBoundsAround(srcLat, srcLng, dstLat, dstLng, marginKm float64) (BoundingBox, bool) {
	marginKm = clampMarginKm(marginKm)
	if marginKm <= 0 {
		return BoundingBox{}, false
	}

	minLat := math.Min(srcLat, dstLat)
	maxLat := math.Max(srcLat, dstLat)
	minLon := math.Min(srcLng, dstLng)
	maxLon := math.Max(srcLng, dstLng)

	centerLat := (srcLat + dstLat) / 2
	latDelta := marginKm / 111.0
	lonScale := math.Cos(centerLat * math.Pi / 180.0)
	if lonScale == 0 {
		lonScale = 1e-6
	}
	lonDelta := marginKm / (111.0 * lonScale)

	return BoundingBox{
		MinLon: minLon - lonDelta,
		MinLat: minLat - latDelta,
		MaxLon: maxLon + lonDelta,
		MaxLat: maxLat + latDelta,
	}, true
}

func clampMarginKm(value float64) float64 {
	switch {
	case value <= 0:
		return 15
	case value < 10:
		return 10
	case value > 25:
		return 25
	default:
		return value
	}
}
