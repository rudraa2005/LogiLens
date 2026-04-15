package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	rctx "github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/models"
)

const defaultAIServiceURL = "http://127.0.0.1:8085"
const defaultAIServiceTimeout = 5 * time.Second
const defaultEdgeFactorCacheTTL = 10 * time.Minute

type RouteInsights struct {
	RiskScore       float64 `json:"risk_score"`
	ConfidenceScore float64 `json:"confidence_score"`
	Explanation     string  `json:"explanation"`
	Available       bool    `json:"available"`
	Fallback        bool    `json:"fallback"`
	Source          string  `json:"source,omitempty"`
	Error           string  `json:"error,omitempty"`
}

type EdgeAIFactors map[string]float64

type AIGateway struct {
	baseURL string
	client  *http.Client

	cacheTTL         time.Duration
	edgeFactorCache  map[string]cachedEdgeFactors
	edgeFactorCacheM sync.RWMutex
}

type aiRouteRequest struct {
	Location      string             `json:"location,omitempty"`
	Region        string             `json:"region,omitempty"`
	TimeBucket    string             `json:"time_bucket,omitempty"`
	Edges         []aiEdge           `json:"edges"`
	Context       aiContext          `json:"context"`
	Route         aiRoute            `json:"route"`
	Alternatives  []aiRoute          `json:"alternatives,omitempty"`
	MLPredictions map[string]float64 `json:"ml_predictions,omitempty"`
}

type cachedEdgeFactors struct {
	expiresAt time.Time
	factors   EdgeAIFactors
}

type aiEdge struct {
	ID       string  `json:"id"`
	From     string  `json:"from,omitempty"`
	To       string  `json:"to,omitempty"`
	ModeID   string  `json:"mode_id,omitempty"`
	Distance float64 `json:"distance,omitempty"`
	Time     float64 `json:"time,omitempty"`
	Cost     float64 `json:"cost,omitempty"`
}

type aiContext struct {
	LocationName string                   `json:"location_name,omitempty"`
	EdgeFactors  map[string]aiEdgeContext `json:"edge_factors"`
}

type aiEdgeContext struct {
	TrafficFactor float64 `json:"traffic_factor,omitempty"`
	WeatherFactor float64 `json:"weather_factor,omitempty"`
	NewsFactor    float64 `json:"news_factor,omitempty"`
}

type aiRoute struct {
	RouteID       string   `json:"route_id,omitempty"`
	Steps         []aiStep `json:"steps"`
	TotalDistance float64  `json:"total_distance"`
	TotalTime     float64  `json:"total_time"`
	TotalCost     float64  `json:"total_cost"`
}

type aiStep struct {
	EdgeID     string  `json:"edge_id,omitempty"`
	FromNodeID string  `json:"from_node_id,omitempty"`
	ToNodeID   string  `json:"to_node_id,omitempty"`
	ModeID     string  `json:"mode_id,omitempty"`
	Distance   float64 `json:"distance,omitempty"`
	Time       float64 `json:"time,omitempty"`
	Cost       float64 `json:"cost,omitempty"`
}

type aiRouteResponse struct {
	RiskScore       float64 `json:"risk_score"`
	ConfidenceScore float64 `json:"confidence_score"`
	Explanation     string  `json:"explanation"`
}

type aiEdgeFactorResponse struct {
	EdgeFactors map[string]float64 `json:"edge_factors"`
}

func NewAIGatewayFromEnv() *AIGateway {
	baseURL := strings.TrimSpace(os.Getenv("AI_SERVICE_URL"))
	if baseURL == "" {
		baseURL = defaultAIServiceURL
	}

	timeout := defaultAIServiceTimeout
	if raw := strings.TrimSpace(os.Getenv("AI_SERVICE_TIMEOUT_SECONDS")); raw != "" {
		if seconds, err := strconv.ParseFloat(raw, 64); err == nil && seconds > 0 {
			timeout = time.Duration(seconds * float64(time.Second))
		}
	}

	cacheTTL := defaultEdgeFactorCacheTTL
	if raw := strings.TrimSpace(os.Getenv("AI_EDGE_FACTOR_CACHE_TTL_SECONDS")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			cacheTTL = time.Duration(seconds) * time.Second
		}
	}

	return &AIGateway{
		baseURL: normalizeBaseURL(baseURL),
		client: &http.Client{
			Timeout: timeout,
		},
		cacheTTL:        cacheTTL,
		edgeFactorCache: make(map[string]cachedEdgeFactors),
	}
}

func (a *AIGateway) GetEdgeFactors(ctx context.Context, edges []models.Edge, routeContext rctx.Context) (EdgeAIFactors, error) {
	fallback := neutralEdgeFactors(edges)
	if a == nil {
		return fallback, errors.New("ai gateway is not configured")
	}
	if a.client == nil {
		a.client = &http.Client{Timeout: defaultAIServiceTimeout}
	}
	if strings.TrimSpace(a.baseURL) == "" {
		return fallback, errors.New("ai service url is empty")
	}
	if a.cacheTTL <= 0 {
		a.cacheTTL = defaultEdgeFactorCacheTTL
	}
	if a.edgeFactorCache == nil {
		a.edgeFactorCache = make(map[string]cachedEdgeFactors)
	}

	cacheKey := edgeFactorCacheKey(routeContext)
	if cached, ok := a.cachedEdgeFactors(cacheKey); ok {
		return mergeEdgeFactorFallback(fallback, cached), nil
	}

	bucket := rctx.TimeBucket(routeContext.DepartureTime)

	payload := aiRouteRequest{
		Location: routeContext.LocationName,
		Region:   routeContext.LocationName,
		TimeBucket: func() string {
			if bucket.IsZero() {
				return ""
			}
			return bucket.Format(time.RFC3339)
		}(),
		Edges: graphEdges(edges),
		Context: aiContext{
			LocationName: routeContext.LocationName,
			EdgeFactors:  routeEdgeFactors(routeContext),
		},
		Route: aiRoute{},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fallback, err
	}

	endpoint := strings.TrimRight(a.baseURL, "/") + "/edge-factors"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fallback, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fallback, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fallback, fmt.Errorf("ai service returned status %s", resp.Status)
	}

	var decoded aiEdgeFactorResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return fallback, err
	}

	for edgeID := range fallback {
		if value, ok := decoded.EdgeFactors[edgeID]; ok {
			fallback[edgeID] = clampAIFactor(value)
		}
	}
	a.storeEdgeFactors(cacheKey, fallback)
	return fallback, nil
}

func (a *AIGateway) GetRouteInsights(ctx context.Context, route RouteResponse, routeContext rctx.Context) (*RouteInsights, error) {
	if a == nil {
		return fallbackInsights(route, routeContext, errors.New("ai gateway is not configured")), errors.New("ai gateway is not configured")
	}
	if a.client == nil {
		a.client = &http.Client{Timeout: defaultAIServiceTimeout}
	}
	if strings.TrimSpace(a.baseURL) == "" {
		return fallbackInsights(route, routeContext, errors.New("ai service url is empty")), errors.New("ai service url is empty")
	}

	payload := aiRouteRequest{
		Location: buildLocationLabel(route),
		Edges:    routeEdges(route),
		Context: aiContext{
			LocationName: buildLocationLabel(route),
			EdgeFactors:  routeEdgeFactors(routeContext),
		},
		Route: aiRoute{
			RouteID:       route.RouteID,
			Steps:         routeSteps(route),
			TotalDistance: route.TotalDistance,
			TotalTime:     route.TotalTime,
			TotalCost:     route.TotalCost,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fallbackInsights(route, routeContext, err), err
	}

	endpoint := strings.TrimRight(a.baseURL, "/") + "/route-confidence"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fallbackInsights(route, routeContext, err), err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fallbackInsights(route, routeContext, err), err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fallbackInsights(route, routeContext, fmt.Errorf("ai service returned status %s", resp.Status)), fmt.Errorf("ai service returned status %s", resp.Status)
	}

	var decoded aiRouteResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return fallbackInsights(route, routeContext, err), err
	}

	return &RouteInsights{
		RiskScore:       decoded.RiskScore,
		ConfidenceScore: decoded.ConfidenceScore,
		Explanation:     decoded.Explanation,
		Available:       true,
		Fallback:        false,
		Source:          "ai-service",
	}, nil
}

func normalizeBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return strings.TrimRight(raw, "/")
	}
	return strings.TrimRight("http://"+raw, "/")
}

func fallbackInsights(route RouteResponse, routeContext rctx.Context, err error) *RouteInsights {
	risk := routeRiskFromContext(route, routeContext)
	confidence := 100.0 - risk*0.55
	if confidence < 25 {
		confidence = 25
	}

	explanation := "AI service unavailable; route computed with local fallback insights."
	if err != nil {
		explanation = explanation + " " + err.Error()
	}

	return &RouteInsights{
		RiskScore:       round1(risk),
		ConfidenceScore: round1(confidence),
		Explanation:     explanation,
		Available:       false,
		Fallback:        true,
		Source:          "fallback",
		Error:           errString(err),
	}
}

func routeRiskFromContext(route RouteResponse, routeContext rctx.Context) float64 {
	totalWeight := 0.0
	totalRisk := 0.0

	for _, step := range route.Steps {
		factors := routeContext.EdgeContextAt(step.EdgeID, routeContext.DepartureTime)

		traffic := severity(factors.TrafficFactor)
		weather := severity(factors.WeatherFactor)
		news := severity(factors.NewsFactor)
		stepRisk := 100.0 * max3(traffic, weather, news)
		weight := stepWeight(step)
		totalRisk += stepRisk * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 20.0
	}

	return totalRisk / totalWeight
}

func buildLocationLabel(route RouteResponse) string {
	switch {
	case strings.TrimSpace(route.SourceNodeID) != "" && strings.TrimSpace(route.DestinationNodeID) != "":
		return strings.TrimSpace(route.SourceNodeID) + " -> " + strings.TrimSpace(route.DestinationNodeID)
	case strings.TrimSpace(route.RouteID) != "":
		return strings.TrimSpace(route.RouteID)
	default:
		return "selected route"
	}
}

func routeEdges(route RouteResponse) []aiEdge {
	edges := make([]aiEdge, 0, len(route.Steps))
	for _, step := range route.Steps {
		edges = append(edges, aiEdge{
			ID:       step.EdgeID,
			From:     step.FromNodeID,
			To:       step.ToNodeID,
			ModeID:   step.ModeID,
			Distance: step.Distance,
			Time:     step.Time,
			Cost:     step.Cost,
		})
	}
	return edges
}

func graphEdges(edges []models.Edge) []aiEdge {
	out := make([]aiEdge, 0, len(edges))
	for _, edge := range edges {
		out = append(out, aiEdge{
			ID:       edge.ID,
			From:     edge.From,
			To:       edge.To,
			ModeID:   edge.ModeID,
			Distance: edge.Distance,
			Time:     edge.Time,
			Cost:     edge.Cost,
		})
	}
	return out
}

func routeSteps(route RouteResponse) []aiStep {
	steps := make([]aiStep, 0, len(route.Steps))
	for _, step := range route.Steps {
		steps = append(steps, aiStep{
			EdgeID:     step.EdgeID,
			FromNodeID: step.FromNodeID,
			ToNodeID:   step.ToNodeID,
			ModeID:     step.ModeID,
			Distance:   step.Distance,
			Time:       step.Time,
			Cost:       step.Cost,
		})
	}
	return steps
}

func routeEdgeFactors(routeContext rctx.Context) map[string]aiEdgeContext {
	factors := make(map[string]aiEdgeContext, len(routeContext.BaseEdgeFactors))
	for edgeID, item := range routeContext.BaseEdgeFactors {
		factors[edgeID] = aiEdgeContext{
			TrafficFactor: item.TrafficFactor,
			WeatherFactor: item.WeatherFactor,
			NewsFactor:    item.NewsFactor,
		}
	}
	return factors
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func max3(a, b, c float64) float64 {
	if a >= b && a >= c {
		return a
	}
	if b >= c {
		return b
	}
	return c
}

func severity(factor float64) float64 {
	if factor <= 1 {
		return 0
	}
	if factor > 2 {
		factor = 2
	}
	return factor - 1
}

func stepWeight(step models.RouteStep) float64 {
	switch {
	case step.Time > 0:
		return step.Time
	case step.Distance > 0:
		return step.Distance
	default:
		return 1
	}
}

func round1(value float64) float64 {
	return mathRound(value*10) / 10
}

func neutralEdgeFactors(edges []models.Edge) EdgeAIFactors {
	out := make(EdgeAIFactors, len(edges))
	for _, edge := range edges {
		if edge.ID == "" {
			continue
		}
		out[edge.ID] = 1.0
	}
	return out
}

func clampAIFactor(value float64) float64 {
	switch {
	case value <= 0:
		return 1.0
	case value < 0.8:
		return 0.8
	case value > 1.5:
		return 1.5
	default:
		return value
	}
}

func mathRound(value float64) float64 {
	if value < 0 {
		return float64(int(value - 0.5))
	}
	return float64(int(value + 0.5))
}

func edgeFactorCacheKey(routeContext rctx.Context) string {
	region := strings.TrimSpace(strings.ToLower(routeContext.LocationName))
	if region == "" {
		region = "global"
	}
	bucket := rctx.TimeBucket(routeContext.DepartureTime)
	if bucket.IsZero() {
		return region
	}
	return region + "|" + bucket.Format(time.RFC3339)
}

func (a *AIGateway) cachedEdgeFactors(key string) (EdgeAIFactors, bool) {
	if a == nil || key == "" {
		return nil, false
	}
	a.edgeFactorCacheM.RLock()
	entry, ok := a.edgeFactorCache[key]
	a.edgeFactorCacheM.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return cloneEdgeFactors(entry.factors), true
}

func (a *AIGateway) storeEdgeFactors(key string, factors EdgeAIFactors) {
	if a == nil || key == "" || len(factors) == 0 {
		return
	}
	a.edgeFactorCacheM.Lock()
	defer a.edgeFactorCacheM.Unlock()
	if a.edgeFactorCache == nil {
		a.edgeFactorCache = make(map[string]cachedEdgeFactors)
	}
	a.edgeFactorCache[key] = cachedEdgeFactors{
		expiresAt: time.Now().Add(a.cacheTTL),
		factors:   cloneEdgeFactors(factors),
	}
}

func mergeEdgeFactorFallback(fallback EdgeAIFactors, overrides EdgeAIFactors) EdgeAIFactors {
	merged := cloneEdgeFactors(fallback)
	for edgeID, factor := range overrides {
		if _, ok := merged[edgeID]; ok {
			merged[edgeID] = factor
		}
	}
	return merged
}

func cloneEdgeFactors(values EdgeAIFactors) EdgeAIFactors {
	if len(values) == 0 {
		return EdgeAIFactors{}
	}
	cloned := make(EdgeAIFactors, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
