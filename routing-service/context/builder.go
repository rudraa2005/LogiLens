package context

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultRouteMarginKm      = 15.0
	defaultNearbyEdgeRadiusKm = 0.4
	defaultAverageSpeedKPH    = 55.0
	defaultContextCacheTTL    = 10 * time.Minute
)

type EdgeDistance struct {
	EdgeID     string
	DistanceKm float64
}

type ContextConfig struct {
	RouteMarginKm      float64
	NearbyEdgeRadiusKm float64
	AverageSpeedKPH    float64
	CacheTTL           time.Duration
}

type ContextService struct {
	NearestEdge func(lat, lng float64) string
	NearbyEdges func(lat, lng, radiusKm float64) []EdgeDistance
	NodeLabel   func(lat, lng float64) string

	TrafficFetcher TrafficFetcher
	WeatherFetcher WeatherFetcher
	NewsSource     NewsSource
	NewsAnalyzer   NewsAnalyzer

	RouteMarginKm      float64
	NearbyEdgeRadiusKm float64
	AverageSpeedKPH    float64
	CacheTTL           time.Duration

	trafficCache *responseCache
	weatherCache *responseCache
	newsCache    *responseCache
}

type routeAnchor struct {
	lat      float64
	lng      float64
	progress float64
}

type cacheEntry struct {
	expiresAt time.Time
	value     any
}

type responseCache struct {
	mu    sync.RWMutex
	items map[string]cacheEntry
}

func NewContextService(nearestEdge func(lat, lng float64) string, nodeLabel func(lat, lng float64) string) *ContextService {
	return NewContextServiceWithConfig(nearestEdge, nodeLabel, contextConfigFromEnv())
}

func NewContextServiceWithConfig(nearestEdge func(lat, lng float64) string, nodeLabel func(lat, lng float64) string, cfg ContextConfig) *ContextService {
	marginKm := clampMarginKm(cfg.RouteMarginKm)
	if marginKm <= 0 {
		marginKm = defaultRouteMarginKm
	}

	nearbyRadiusKm := cfg.NearbyEdgeRadiusKm
	if nearbyRadiusKm <= 0 {
		nearbyRadiusKm = defaultNearbyEdgeRadiusKm
	}

	averageSpeedKPH := cfg.AverageSpeedKPH
	if averageSpeedKPH <= 0 {
		averageSpeedKPH = defaultAverageSpeedKPH
	}

	cacheTTL := cfg.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = defaultContextCacheTTL
	}

	return &ContextService{
		NearestEdge:        nearestEdge,
		NodeLabel:          nodeLabel,
		TrafficFetcher:     NewTomTomTrafficFetcher(),
		WeatherFetcher:     NewOpenMeteoWeatherFetcher(),
		NewsSource:         NewGoogleNewsSource(),
		NewsAnalyzer:       NewOpenAINewsAnalyzer(),
		RouteMarginKm:      marginKm,
		NearbyEdgeRadiusKm: nearbyRadiusKm,
		AverageSpeedKPH:    averageSpeedKPH,
		CacheTTL:           cacheTTL,
		trafficCache:       newResponseCache(),
		weatherCache:       newResponseCache(),
		newsCache:          newResponseCache(),
	}
}

func contextConfigFromEnv() ContextConfig {
	marginKm := defaultRouteMarginKm
	if raw := firstNonEmpty(os.Getenv("CONTEXT_ROUTE_MARGIN_KM"), os.Getenv("ROUTE_MARGIN_KM")); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			marginKm = parsed
		}
	}

	nearbyRadiusKm := defaultNearbyEdgeRadiusKm
	if raw := strings.TrimSpace(os.Getenv("CONTEXT_NEARBY_EDGE_RADIUS_KM")); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 {
			nearbyRadiusKm = parsed
		}
	}

	averageSpeedKPH := defaultAverageSpeedKPH
	if raw := strings.TrimSpace(os.Getenv("CONTEXT_AVG_SPEED_KPH")); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed > 0 {
			averageSpeedKPH = parsed
		}
	}

	cacheTTL := defaultContextCacheTTL
	if raw := strings.TrimSpace(os.Getenv("CONTEXT_CACHE_TTL_SECONDS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			cacheTTL = time.Duration(parsed) * time.Second
		}
	}

	return ContextConfig{
		RouteMarginKm:      marginKm,
		NearbyEdgeRadiusKm: nearbyRadiusKm,
		AverageSpeedKPH:    averageSpeedKPH,
		CacheTTL:           cacheTTL,
	}
}

func (c *ContextService) BuildForRoute(srcLat, srcLng, dstLat, dstLng float64, departureTime time.Time) Context {
	ctx := BuildContext()
	if c == nil {
		return ctx
	}

	if departureTime.IsZero() {
		departureTime = time.Now().UTC()
	}
	ctx.DepartureTime = departureTime

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
	if c.NodeLabel != nil {
		ctx.LocationName = strings.TrimSpace(c.NodeLabel(routeLat, routeLng))
	}

	anchors := []routeAnchor{
		{lat: srcLat, lng: srcLng, progress: 0},
		{lat: routeLat, lng: routeLng, progress: 0.5},
		{lat: dstLat, lng: dstLng, progress: 1},
	}
	baseTravelDuration := estimateTravelDuration(srcLat, srcLng, dstLat, dstLng, c.AverageSpeedKPH)

	var wg sync.WaitGroup
	var trafficSignals []TrafficSignal
	var newsFactor float64
	weatherFactors := make([]float64, len(anchors))

	if c.TrafficFetcher != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			signals, err := c.fetchTraffic(bounds, departureTime)
			if err == nil {
				trafficSignals = signals
			}
		}()
	}

	if c.NewsSource != nil && c.NewsAnalyzer != nil {
		query := ctx.LocationName
		wg.Add(1)
		go func() {
			defer wg.Done()
			factor, err := c.fetchNews(query)
			if err == nil {
				newsFactor = factor
			}
		}()
	}

	if c.WeatherFetcher != nil {
		for i, anchor := range anchors {
			wg.Add(1)
			go func(index int, anchor routeAnchor) {
				defer wg.Done()
				eta := departureTime.Add(scaleDuration(baseTravelDuration, anchor.progress))
				factor, err := c.fetchWeather(anchor.lat, anchor.lng, eta)
				if err == nil {
					weatherFactors[index] = factor
				}
			}(i, anchor)
		}
	}

	wg.Wait()

	for _, signal := range trafficSignals {
		eta := estimateArrivalTime(srcLat, srcLng, dstLat, dstLng, signal.Latitude, signal.Longitude, departureTime, baseTravelDuration)
		factor := signal.Factor * timeOfDayTrafficMultiplier(eta)
		matches := c.mapSignalMatches(signal.Latitude, signal.Longitude, c.NearbyEdgeRadiusKm, nearestEdge)
		distributeFactor(&ctx, matches, EdgeContext{TrafficFactor: factor}, eta)
	}

	for i, anchor := range anchors {
		if weatherFactors[i] <= 0 {
			continue
		}
		eta := departureTime.Add(scaleDuration(baseTravelDuration, anchor.progress))
		matches := c.mapSignalMatches(anchor.lat, anchor.lng, maxFloat(c.NearbyEdgeRadiusKm*2, 0.8), nearestEdge)
		distributeFactor(&ctx, matches, EdgeContext{WeatherFactor: weatherFactors[i]}, eta)
	}

	if newsFactor > 0 {
		for _, anchor := range anchors {
			eta := departureTime.Add(scaleDuration(baseTravelDuration, anchor.progress))
			matches := c.mapSignalMatches(anchor.lat, anchor.lng, maxFloat(c.NearbyEdgeRadiusKm*3, 1.2), nearestEdge)
			distributeFactor(&ctx, matches, EdgeContext{NewsFactor: newsFactor}, eta)
		}
	}

	return ctx
}

func (c *ContextService) MapSignalToEdges(signalLat, signalLng float64) []string {
	matches := c.mapSignalMatches(signalLat, signalLng, c.NearbyEdgeRadiusKm, c.NearestEdge)
	edgeIDs := make([]string, 0, len(matches))
	for _, match := range matches {
		edgeIDs = append(edgeIDs, match.EdgeID)
	}
	return edgeIDs
}

func (c *ContextService) mapSignalMatches(lat, lng, radiusKm float64, fallback func(lat, lng float64) string) []EdgeDistance {
	matches := make([]EdgeDistance, 0)
	seen := make(map[string]struct{})

	if c != nil && c.NearbyEdges != nil {
		for _, match := range c.NearbyEdges(lat, lng, radiusKm) {
			match.EdgeID = strings.TrimSpace(match.EdgeID)
			if match.EdgeID == "" {
				continue
			}
			if _, ok := seen[match.EdgeID]; ok {
				continue
			}
			seen[match.EdgeID] = struct{}{}
			matches = append(matches, match)
		}
	}

	if len(matches) > 0 {
		return matches
	}

	if fallback != nil {
		if edgeID := strings.TrimSpace(fallback(lat, lng)); edgeID != "" {
			return []EdgeDistance{{EdgeID: edgeID, DistanceKm: 0}}
		}
	}

	return nil
}

func applyFactor(ctx *Context, edgeID string, delta EdgeContext) {
	ctx.EdgeFactors[edgeID] = mergeEdgeContext(ctx.EdgeFactors[edgeID], delta)
}

func distributeFactor(ctx *Context, matches []EdgeDistance, delta EdgeContext, arrival time.Time) {
	if len(matches) == 0 {
		return
	}

	weights := inverseDistanceWeights(matches)
	for _, match := range matches {
		weight := weights[match.EdgeID]
		scaled := EdgeContext{
			TrafficFactor: scaledFactor(delta.TrafficFactor, weight),
			WeatherFactor: scaledFactor(delta.WeatherFactor, weight),
			NewsFactor:    scaledFactor(delta.NewsFactor, weight),
			AIFactor:      scaledFactor(delta.AIFactor, weight),
		}
		applyFactor(ctx, match.EdgeID, scaled)
		if !arrival.IsZero() {
			ctx.AddTimedEdgeFactor(match.EdgeID, arrival, scaled)
			if existing, ok := ctx.EdgeArrivalTimes[match.EdgeID]; !ok || arrival.Before(existing) {
				ctx.EdgeArrivalTimes[match.EdgeID] = arrival
			}
		}
	}
}

func inverseDistanceWeights(matches []EdgeDistance) map[string]float64 {
	if len(matches) == 1 {
		return map[string]float64{matches[0].EdgeID: 1}
	}

	total := 0.0
	raw := make(map[string]float64, len(matches))
	for _, match := range matches {
		weight := 1.0 / maxFloat(match.DistanceKm, 0.05)
		raw[match.EdgeID] = weight
		total += weight
	}
	if total <= 0 {
		total = float64(len(matches))
	}

	weights := make(map[string]float64, len(matches))
	for edgeID, weight := range raw {
		weights[edgeID] = weight / total
	}
	return weights
}

func scaledFactor(factor, weight float64) float64 {
	if factor <= 0 {
		return 0
	}
	if weight <= 0 {
		return 1.0
	}
	return 1.0 + (factor-1.0)*weight
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
		return defaultRouteMarginKm
	case value < 10:
		return 10
	case value > 25:
		return 25
	default:
		return value
	}
}

func estimateTravelDuration(srcLat, srcLng, dstLat, dstLng, averageSpeedKPH float64) time.Duration {
	distanceKm := haversineDistanceKm(srcLat, srcLng, dstLat, dstLng)
	if averageSpeedKPH <= 0 {
		averageSpeedKPH = defaultAverageSpeedKPH
	}

	hours := distanceKm / averageSpeedKPH
	if hours < 0.25 {
		hours = 0.25
	}
	return time.Duration(hours * float64(time.Hour))
}

func estimateArrivalTime(srcLat, srcLng, dstLat, dstLng, pointLat, pointLng float64, departureTime time.Time, travelDuration time.Duration) time.Time {
	progress := pointProgressAlongRoute(srcLat, srcLng, dstLat, dstLng, pointLat, pointLng)
	return departureTime.Add(scaleDuration(travelDuration, progress))
}

func pointProgressAlongRoute(srcLat, srcLng, dstLat, dstLng, pointLat, pointLng float64) float64 {
	ax, ay := projectToLocalKm(srcLat, srcLng, srcLat, srcLng)
	bx, by := projectToLocalKm(srcLat, srcLng, dstLat, dstLng)
	px, py := projectToLocalKm(srcLat, srcLng, pointLat, pointLng)

	dx := bx - ax
	dy := by - ay
	denom := dx*dx + dy*dy
	if denom == 0 {
		return 0.5
	}

	progress := ((px-ax)*dx + (py-ay)*dy) / denom
	switch {
	case progress < 0:
		return 0
	case progress > 1:
		return 1
	default:
		return progress
	}
}

func scaleDuration(base time.Duration, progress float64) time.Duration {
	if progress <= 0 {
		return 0
	}
	if progress >= 1 {
		return base
	}
	return time.Duration(float64(base) * progress)
}

func timeOfDayTrafficMultiplier(at time.Time) float64 {
	hour := at.In(time.UTC).Hour()
	switch {
	case hour >= 6 && hour < 10:
		return 1.15
	case hour >= 16 && hour < 20:
		return 1.20
	case hour >= 22 || hour < 5:
		return 0.95
	default:
		return 1.0
	}
}

func (c *ContextService) fetchTraffic(bounds BoundingBox, at time.Time) ([]TrafficSignal, error) {
	if c == nil || c.TrafficFetcher == nil {
		return nil, nil
	}

	key := fmt.Sprintf("%s:%s", bounds.String(), at.UTC().Format("2006-01-02T15"))
	if cached, ok := c.trafficCache.get(key, time.Now()); ok {
		if signals, ok := cached.([]TrafficSignal); ok {
			return append([]TrafficSignal(nil), signals...), nil
		}
	}

	signals, err := c.TrafficFetcher.Fetch(bounds, at)
	if err != nil {
		return nil, err
	}

	c.trafficCache.set(key, append([]TrafficSignal(nil), signals...), time.Now().Add(c.CacheTTL))
	return signals, nil
}

func (c *ContextService) fetchWeather(lat, lng float64, at time.Time) (float64, error) {
	if c == nil || c.WeatherFetcher == nil {
		return 1.0, nil
	}

	key := fmt.Sprintf("%.4f:%.4f:%s", lat, lng, at.UTC().Format("2006-01-02T15"))
	if cached, ok := c.weatherCache.get(key, time.Now()); ok {
		if factor, ok := cached.(float64); ok {
			return factor, nil
		}
	}

	factor, err := c.WeatherFetcher.Fetch(lat, lng, at)
	if err != nil {
		return 1.0, err
	}

	c.weatherCache.set(key, factor, time.Now().Add(c.CacheTTL))
	return factor, nil
}

func (c *ContextService) fetchNews(query string) (float64, error) {
	query = strings.TrimSpace(query)
	if c == nil || c.NewsSource == nil || c.NewsAnalyzer == nil || query == "" {
		return 1.0, nil
	}

	key := strings.ToLower(query)
	if cached, ok := c.newsCache.get(key, time.Now()); ok {
		if factor, ok := cached.(float64); ok {
			return factor, nil
		}
	}

	headlines, err := c.NewsSource.Fetch(query)
	if err != nil {
		return 1.0, err
	}
	factor, err := c.NewsAnalyzer.Analyze(headlines)
	if err != nil {
		return 1.0, err
	}

	c.newsCache.set(key, factor, time.Now().Add(c.CacheTTL))
	return factor, nil
}

func newResponseCache() *responseCache {
	return &responseCache{items: make(map[string]cacheEntry)}
}

func (c *responseCache) get(key string, now time.Time) (any, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || now.After(entry.expiresAt) {
		return nil, false
	}
	return entry.value, true
}

func (c *responseCache) set(key string, value any, expiresAt time.Time) {
	if c == nil {
		return
	}

	c.mu.Lock()
	c.items[key] = cacheEntry{
		expiresAt: expiresAt,
		value:     value,
	}
	c.mu.Unlock()
}

func haversineDistanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := degreesToRadians(lat2 - lat1)
	dLon := degreesToRadians(lon2 - lon1)

	lat1Rad := degreesToRadians(lat1)
	lat2Rad := degreesToRadians(lat2)

	sinLat := math.Sin(dLat / 2)
	sinLon := math.Sin(dLon / 2)

	a := sinLat*sinLat + math.Cos(lat1Rad)*math.Cos(lat2Rad)*sinLon*sinLon
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return 6371.0 * c
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180.0
}

func projectToLocalKm(originLat, originLng, lat, lng float64) (float64, float64) {
	latKm := degreesToRadians(lat-originLat) * 6371.0
	lonScale := math.Cos(degreesToRadians(originLat))
	lonKm := degreesToRadians(lng-originLng) * 6371.0 * lonScale
	return lonKm, latKm
}
