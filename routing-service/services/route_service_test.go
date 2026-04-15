package services

import (
	"context"
	"errors"
	"testing"

	"github.com/rudraa2005/LogiLens/routing-service/comparison"
	rctx "github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/graph"
	"github.com/rudraa2005/LogiLens/routing-service/models"
	"github.com/rudraa2005/LogiLens/routing-service/repository"
)

type fakeGeocoder struct {
	calls  []string
	coords map[string][2]float64
	err    error
}

func (f *fakeGeocoder) Geocode(place string) (float64, float64, error) {
	f.calls = append(f.calls, place)
	if f.err != nil {
		return 0, 0, f.err
	}

	coords, ok := f.coords[place]
	if !ok {
		return 0, 0, errors.New("unexpected place")
	}

	return coords[0], coords[1], nil
}

type fakeRouteRepo struct {
	route models.Route
	steps []models.RouteStep
}

type fakeContextBuilder struct {
	ctx rctx.Context
}

func (f *fakeContextBuilder) BuildForRoute(srcLat, srcLng, dstLat, dstLng float64) rctx.Context {
	return f.ctx
}

type fakeAIGateway struct {
	insights *RouteInsights
	err      error
	calls    int
}

func (f *fakeAIGateway) GetRouteInsights(ctx context.Context, route RouteResponse, routeContext rctx.Context) (*RouteInsights, error) {
	f.calls++
	if f.err != nil && f.insights == nil {
		return nil, f.err
	}
	return f.insights, f.err
}

func (f *fakeRouteRepo) CreateRoute(ctx context.Context, route models.Route) (string, error) {
	f.route = route
	return route.ID, nil
}

func (f *fakeRouteRepo) CreateRouteSteps(ctx context.Context, routeID string, steps []models.RouteStep) error {
	f.steps = append([]models.RouteStep(nil), steps...)
	return nil
}

func TestComputeRouteUsesGeocodeThenNearestNodeThenAstar(t *testing.T) {
	g := graph.BuildGraph(
		[]models.Node{
			{NodeID: "source-node", Name: "source-node", Latitude: 12.0, Longitude: 77.0},
			{NodeID: "dest-node", Name: "dest-node", Latitude: 13.0, Longitude: 78.0},
		},
		[]models.Edge{
			{
				ID:       "edge-1",
				From:     "source-node",
				To:       "dest-node",
				ModeID:   "truck",
				Distance: 10,
				Time:     12,
				Cost:     5,
				Geometry: []models.LatLng{{Latitude: 12.0, Longitude: 77.0}, {Latitude: 13.0, Longitude: 78.0}},
			},
		},
	)

	geo := &fakeGeocoder{
		coords: map[string][2]float64{
			"source place":      {12.01, 77.01},
			"destination place": {12.99, 77.99},
		},
	}
	repo := &fakeRouteRepo{}
	svc := &RouteService{
		repo:           repo,
		graph:          g,
		geocoder:       geo,
		contextBuilder: nil,
	}

	resp, err := svc.ComputeRoute(context.Background(), RouteRequest{
		Source:      "source place",
		Destination: "destination place",
		OptimizeBy:  "time",
	})
	if err != nil {
		t.Fatalf("ComputeRoute returned error: %v", err)
	}

	if got := len(geo.calls); got != 2 {
		t.Fatalf("expected 2 geocode calls, got %d", got)
	}
	if geo.calls[0] != "source place" || geo.calls[1] != "destination place" {
		t.Fatalf("unexpected geocode call order: %#v", geo.calls)
	}

	if repo.route.SourceNodeID != "source-node" {
		t.Fatalf("expected source-node, got %q", repo.route.SourceNodeID)
	}
	if repo.route.DestinationNodeID != "dest-node" {
		t.Fatalf("expected dest-node, got %q", repo.route.DestinationNodeID)
	}
	if resp.RouteID != repo.route.ID {
		t.Fatalf("expected route id %q, got %q", repo.route.ID, resp.RouteID)
	}
	if len(resp.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(resp.Steps))
	}
}

func TestComputeRouteMergesAIInsightsWithoutBlocking(t *testing.T) {
	g := graph.BuildGraph(
		[]models.Node{
			{NodeID: "source-node", Name: "source-node", Latitude: 12.0, Longitude: 77.0},
			{NodeID: "dest-node", Name: "dest-node", Latitude: 13.0, Longitude: 78.0},
		},
		[]models.Edge{
			{
				ID:       "edge-1",
				From:     "source-node",
				To:       "dest-node",
				ModeID:   "truck",
				Distance: 10,
				Time:     12,
				Cost:     5,
				Geometry: []models.LatLng{{Latitude: 12.0, Longitude: 77.0}, {Latitude: 13.0, Longitude: 78.0}},
			},
		},
	)

	geo := &fakeGeocoder{
		coords: map[string][2]float64{
			"source place":      {12.01, 77.01},
			"destination place": {12.99, 77.99},
		},
	}
	repo := &fakeRouteRepo{}
	ctxBuilder := &fakeContextBuilder{
		ctx: rctx.Context{
			EdgeFactors: map[string]rctx.EdgeContext{
				"edge-1": {TrafficFactor: 1.4, WeatherFactor: 1.1, NewsFactor: 1.6},
			},
		},
	}
	aiGateway := &fakeAIGateway{
		insights: &RouteInsights{
			RiskScore:       64.2,
			ConfidenceScore: 51.4,
			Explanation:     "Live AI insights attached.",
			Available:       true,
			Fallback:        false,
			Source:          "ai-service",
		},
	}
	svc := &RouteService{
		repo:           repo,
		graph:          g,
		geocoder:       geo,
		contextBuilder: ctxBuilder,
		aiGateway:      aiGateway,
	}

	resp, err := svc.ComputeRoute(context.Background(), RouteRequest{
		Source:      "source place",
		Destination: "destination place",
		OptimizeBy:  "time",
	})
	if err != nil {
		t.Fatalf("ComputeRoute returned error: %v", err)
	}

	if aiGateway.calls != 1 {
		t.Fatalf("expected AI gateway to be called once, got %d", aiGateway.calls)
	}
	if resp.Insights == nil {
		t.Fatal("expected AI insights to be merged into the route response")
	}
	if !resp.Insights.Available {
		t.Fatalf("expected insights to be marked available, got %+v", resp.Insights)
	}
	if resp.Insights.Source != "ai-service" {
		t.Fatalf("expected ai-service source, got %+v", resp.Insights)
	}
}

func TestRouteResponseToProtoIncludesAlternativesAndInsights(t *testing.T) {
	resp := RouteResponse{
		RouteID:       "route-1",
		TotalDistance: 12.4,
		TotalTime:     18.6,
		TotalCost:     5.2,
		Steps: []models.RouteStep{
			{
				FromNodeID: "source",
				ToNodeID:   "target",
				EdgeID:     "edge-1",
				ModeID:     "truck",
				Distance:   12.4,
				Time:       18.6,
				Cost:       5.2,
			},
		},
		Alternatives: []comparison.Route{
			{
				RouteID:       "alt-1",
				TotalDistance: 14.0,
				TotalTime:     21.0,
				TotalCost:     6.3,
				Steps: []models.RouteStep{
					{
						FromNodeID: "source",
						ToNodeID:   "target",
						EdgeID:     "edge-2",
						ModeID:     "truck",
						Distance:   14.0,
						Time:       21.0,
						Cost:       6.3,
					},
				},
			},
		},
		Explanation:     "AI-backed route explanation",
		TimeSaved:       2.4,
		CostSaved:       1.1,
		ConfidenceScore: 82.3,
	}

	protoResp := resp.ToProto()
	if protoResp.GetExplanation() != resp.Explanation {
		t.Fatalf("expected explanation %q, got %q", resp.Explanation, protoResp.GetExplanation())
	}
	if protoResp.GetTimeSaved() != resp.TimeSaved {
		t.Fatalf("expected time saved %v, got %v", resp.TimeSaved, protoResp.GetTimeSaved())
	}
	if protoResp.GetCostSaved() != resp.CostSaved {
		t.Fatalf("expected cost saved %v, got %v", resp.CostSaved, protoResp.GetCostSaved())
	}
	if protoResp.GetConfidenceScore() != resp.ConfidenceScore {
		t.Fatalf("expected confidence score %v, got %v", resp.ConfidenceScore, protoResp.GetConfidenceScore())
	}
	if len(protoResp.GetAlternatives()) != 1 {
		t.Fatalf("expected 1 alternative, got %d", len(protoResp.GetAlternatives()))
	}
	if protoResp.GetAlternatives()[0].GetRouteId() != "alt-1" {
		t.Fatalf("unexpected alternative route id: %q", protoResp.GetAlternatives()[0].GetRouteId())
	}
	if len(protoResp.GetAlternatives()[0].GetSteps()) != 1 {
		t.Fatalf("expected 1 step in alternative, got %d", len(protoResp.GetAlternatives()[0].GetSteps()))
	}
}

func TestComputeRoutesReturnsMultipleAlternatives(t *testing.T) {
	g := graph.BuildGraph(
		[]models.Node{
			{NodeID: "s", Latitude: 0, Longitude: 0},
			{NodeID: "a", Latitude: 0, Longitude: 1},
			{NodeID: "b", Latitude: 1, Longitude: 0},
			{NodeID: "t", Latitude: 1, Longitude: 1},
		},
		[]models.Edge{
			{ID: "s-a", From: "s", To: "a", Distance: 1, Time: 1, Cost: 1},
			{ID: "a-t", From: "a", To: "t", Distance: 1, Time: 1, Cost: 1},
			{ID: "s-b", From: "s", To: "b", Distance: 1, Time: 2, Cost: 2},
			{ID: "b-t", From: "b", To: "t", Distance: 1, Time: 2, Cost: 2},
		},
	)

	geo := &fakeGeocoder{
		coords: map[string][2]float64{
			"source place":      {0, 0},
			"destination place": {1, 1},
		},
	}
	svc := &RouteService{
		graph:          g,
		geocoder:       geo,
		repo:           &fakeRouteRepo{},
		contextBuilder: nil,
	}

	routes, err := svc.ComputeRoutes(context.Background(), RouteRequest{
		Source:      "source place",
		Destination: "destination place",
		OptimizeBy:  "time",
	}, 5)
	if err != nil {
		t.Fatalf("ComputeRoutes returned error: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if routes[0].TotalTime != 2 || routes[1].TotalTime != 4 {
		t.Fatalf("unexpected route totals: %#v", routes)
	}
}

func TestResolveNearestNodeRejectsMissingGraph(t *testing.T) {
	svc := &RouteService{geocoder: &fakeGeocoder{}}

	_, _, _, err := svc.resolveNearestNode("any place")
	if err == nil {
		t.Fatal("expected error when graph is missing")
	}
}

func TestNewRouteServiceHandlesNilGraph(t *testing.T) {
	svc := NewRouteService((*repository.RouteRepository)(nil), nil, &fakeGeocoder{})
	if svc == nil {
		t.Fatal("expected service instance")
	}
	if svc.contextBuilder != nil {
		t.Fatal("expected nil context builder for nil graph")
	}
}
