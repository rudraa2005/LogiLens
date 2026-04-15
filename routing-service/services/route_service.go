package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	LogiLens "github.com/rudraa2005/LogiLens/proto"
	"github.com/rudraa2005/LogiLens/routing-service/comparison"
	rctx "github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/explanation"
	"github.com/rudraa2005/LogiLens/routing-service/graph"
	"github.com/rudraa2005/LogiLens/routing-service/models"
	"github.com/rudraa2005/LogiLens/routing-service/repository"
)

type RouteService struct {
	repo           routeRepository
	graph          *graph.Graph
	contextBuilder contextBuilder
	geocoder       Geocoder
	aiGateway      routeInsightGateway
}

const defaultRouteAlternatives = 5

func NewRouteService(repo *repository.RouteRepository, graph *graph.Graph, geocoder Geocoder) *RouteService {
	var builder contextBuilder
	if graph != nil {
		contextSvc := rctx.NewContextService(graph.FindNearestEdge, func(lat, lng float64) string {
			nodeID := graph.FindNearestNode(lat, lng)
			if nodeID == "" {
				return ""
			}
			node, ok := graph.Nodes[nodeID]
			if !ok {
				return nodeID
			}
			if strings.TrimSpace(node.Name) != "" {
				return node.Name
			}
			return nodeID
		})
		contextSvc.NearbyEdges = graph.FindNearbyEdges
		builder = contextSvc
	}

	return &RouteService{
		repo:           repo,
		graph:          graph,
		contextBuilder: builder,
		geocoder:       geocoder,
		aiGateway:      NewAIGatewayFromEnv(),
	}
}

type routeRepository interface {
	CreateRoute(ctx context.Context, route models.Route) (string, error)
	CreateRouteSteps(ctx context.Context, routeID string, steps []models.RouteStep) error
}

type contextBuilder interface {
	BuildForRoute(srcLat, srcLng, dstLat, dstLng float64) rctx.Context
}

type Geocoder interface {
	Geocode(place string) (float64, float64, error)
}

type routeInsightGateway interface {
	GetRouteInsights(ctx context.Context, route RouteResponse, routeContext rctx.Context) (*RouteInsights, error)
}

type RouteRequest struct {
	Source      string
	Destination string
	OptimizeBy  string
}

type RouteResponse struct {
	SourceNodeID      string
	DestinationNodeID string

	Steps []models.RouteStep

	Polyline []models.LatLng `json:"polyline"`

	Alternatives []comparison.Route

	RouteID       string
	TotalDistance float64
	TotalTime     float64
	TotalCost     float64

	Explanation     string
	TimeSaved       float64
	CostSaved       float64
	ConfidenceScore float64

	Insights *RouteInsights `json:"insights,omitempty"`
}

// flattenStepGeometry concatenates the geometry from every step into a
// single polyline, deduplicating shared endpoints between consecutive steps.
func flattenStepGeometry(steps []models.RouteStep) []models.LatLng {
	var polyline []models.LatLng
	for _, step := range steps {
		for j, pt := range step.Geometry {
			// Skip the first point of subsequent steps when it duplicates
			// the last point of the previous step.
			if j == 0 && len(polyline) > 0 {
				last := polyline[len(polyline)-1]
				if last.Latitude == pt.Latitude && last.Longitude == pt.Longitude {
					continue
				}
			}
			polyline = append(polyline, pt)
		}
	}
	return polyline
}

func (rs *RouteService) resolveNearestNode(place string) (string, float64, float64, error) {
	if rs.geocoder == nil {
		return "", 0, 0, errors.New("geocoder is not configured")
	}
	if rs.graph == nil {
		return "", 0, 0, errors.New("routing graph is not configured")
	}

	lat, lng, err := rs.geocoder.Geocode(place)
	if err != nil {
		return "", 0, 0, err
	}

	nodeID := rs.graph.FindNearestNode(lat, lng)
	if nodeID == "" {
		return "", 0, 0, fmt.Errorf("no nearest node found for %q", place)
	}

	return nodeID, lat, lng, nil
}

func (rs *RouteService) BuildRoute(path []string, edges map[string]models.Edge) []models.RouteStep {
	steps := []models.RouteStep{}

	for i := 0; i < len(path)-1; i++ {
		from := path[i]
		to := path[i+1]

		edge := edges[to]

		step := models.RouteStep{
			FromNodeID: from,
			ToNodeID:   to,
			EdgeID:     edge.ID,
			ModeID:     edge.ModeID,
			Distance:   edge.Distance,
			Time:       edge.Time,
			Cost:       edge.Cost,
			Geometry:   edge.Geometry,
		}

		steps = append(steps, step)
	}
	return steps
}

func (rs *RouteService) CreateRoute(ctx context.Context, route models.Route, steps []models.RouteStep) (string, error) {
	routeID, err := rs.repo.CreateRoute(ctx, route)
	if err != nil {
		return "", err
	}

	err = rs.repo.CreateRouteSteps(ctx, routeID, steps)
	if err != nil {
		return "", err
	}
	return routeID, nil
}

func (rs *RouteService) ComputeRoute(ctx context.Context, req RouteRequest) (RouteResponse, error) {
	routes, routeContext, err := rs.computeRoutes(ctx, req, defaultRouteAlternatives)
	if err != nil {
		return RouteResponse{}, err
	}
	if len(routes) == 0 {
		return RouteResponse{}, errors.New("no route found")
	}

	decision := comparison.CompareRoutes(routeResponsesToComparison(routes))
	best := routeResponseFromComparison(decision.BestRoute)
	best.Alternatives = append([]comparison.Route(nil), decision.Alternatives...)
	best.TimeSaved = decision.TimeSaved
	best.CostSaved = decision.CostSaved
	best.Explanation = explanation.ExplainRoute(decision.BestRoute, decision.Alternatives, routeContext)

	routeID, err := rs.persistRoute(ctx, best)
	if err != nil {
		return RouteResponse{}, err
	}

	best.RouteID = routeID
	if rs.aiGateway != nil {
		if insights, insightErr := rs.aiGateway.GetRouteInsights(ctx, best, routeContext); insights != nil {
			best.Insights = insights
			best.ConfidenceScore = insights.ConfidenceScore
			if strings.TrimSpace(insights.Explanation) != "" {
				best.Explanation = insights.Explanation
			}
			if insightErr != nil && best.Insights.Explanation == "" {
				best.Insights.Explanation = "AI service unavailable; route returned with local fallback insights."
			}
		}
	}
	return best, nil
}

func (rs *RouteService) ComputeRoutes(ctx context.Context, req RouteRequest, limit int) ([]RouteResponse, error) {
	routes, _, err := rs.computeRoutes(ctx, req, limit)
	return routes, err
}

func (rs *RouteService) computeRoutes(ctx context.Context, req RouteRequest, limit int) ([]RouteResponse, rctx.Context, error) {
	sourceID, sourceLat, sourceLng, err := rs.resolveNearestNode(req.Source)
	if err != nil {
		return nil, rctx.BuildContext(), err
	}

	destinationID, destLat, destLng, err := rs.resolveNearestNode(req.Destination)
	if err != nil {
		return nil, rctx.BuildContext(), err
	}

	ctxData := rctx.BuildContext()
	if rs.contextBuilder != nil {
		ctxData = rs.contextBuilder.BuildForRoute(sourceLat, sourceLng, destLat, destLng)
	}

	optimizeBy := req.OptimizeBy
	if optimizeBy == "" {
		optimizeBy = "time"
	}

	paths := rs.graph.KShortestPaths(sourceID, destinationID, ctxData, optimizeBy, limit)
	if len(paths) == 0 {
		return nil, ctxData, errors.New("no route found")
	}

	return rs.routeResponsesFromPaths(paths, sourceID, destinationID), ctxData, nil
}

func (rs *RouteService) routeResponsesFromPaths(paths []graph.PathResult, sourceID, destinationID string) []RouteResponse {
	routes := make([]RouteResponse, 0, len(paths))
	for _, path := range paths {
		steps := rs.BuildRoute(path.Nodes, path.Edges)

		var totalDistance, totalTime, totalCost float64
		for _, step := range steps {
			totalDistance += step.Distance
			totalCost += step.Cost
			totalTime += step.Time
		}

		routes = append(routes, RouteResponse{
			SourceNodeID:      sourceID,
			DestinationNodeID: destinationID,
			Steps:             steps,
			Polyline:          flattenStepGeometry(steps),
			TotalDistance:     totalDistance,
			TotalTime:         totalTime,
			TotalCost:         totalCost,
		})
	}
	return routes
}

func (rs *RouteService) persistRoute(ctx context.Context, routeResp RouteResponse) (string, error) {
	route := models.Route{
		ID: uuid.New().String(),

		SourceNodeID:      routeResp.SourceNodeID,
		DestinationNodeID: routeResp.DestinationNodeID,

		TotalDistance: routeResp.TotalDistance,
		TotalTime:     routeResp.TotalTime,
		TotalCost:     routeResp.TotalCost,
	}

	dbSteps := []models.RouteStep{}
	for i, step := range routeResp.Steps {
		dbSteps = append(dbSteps, models.RouteStep{
			ID:         uuid.New().String(),
			RouteID:    route.ID,
			FromNodeID: step.FromNodeID,
			ToNodeID:   step.ToNodeID,
			EdgeID:     step.EdgeID,

			Sequence: i,
			Distance: step.Distance,
			Time:     step.Time,
			Cost:     step.Cost,

			ModeID: step.ModeID,
		})
	}

	return rs.CreateRoute(ctx, route, dbSteps)
}

func routeResponsesToComparison(routes []RouteResponse) []comparison.Route {
	out := make([]comparison.Route, 0, len(routes))
	for _, route := range routes {
		out = append(out, routeResponseToComparison(route))
	}
	return out
}

func routeResponseToComparison(route RouteResponse) comparison.Route {
	return comparison.Route{
		RouteID:           route.RouteID,
		SourceNodeID:      route.SourceNodeID,
		DestinationNodeID: route.DestinationNodeID,
		Steps:             append([]models.RouteStep(nil), route.Steps...),
		Polyline:          append([]models.LatLng(nil), route.Polyline...),
		TotalDistance:     route.TotalDistance,
		TotalTime:         route.TotalTime,
		TotalCost:         route.TotalCost,
	}
}

func routeResponseFromComparison(route comparison.Route) RouteResponse {
	return RouteResponse{
		SourceNodeID:      route.SourceNodeID,
		DestinationNodeID: route.DestinationNodeID,
		RouteID:           route.RouteID,
		Steps:             append([]models.RouteStep(nil), route.Steps...),
		Polyline:          append([]models.LatLng(nil), route.Polyline...),
		TotalDistance:     route.TotalDistance,
		TotalTime:         route.TotalTime,
		TotalCost:         route.TotalCost,
	}
}

func (r RouteResponse) ToProto() *LogiLens.RouteResponse {
	resp := &LogiLens.RouteResponse{
		RouteId:         r.RouteID,
		TotalDistance:   r.TotalDistance,
		TotalTime:       r.TotalTime,
		TotalCost:       r.TotalCost,
		Explanation:     r.Explanation,
		TimeSaved:       r.TimeSaved,
		CostSaved:       r.CostSaved,
		ConfidenceScore: r.ConfidenceScore,
	}

	for _, alt := range r.Alternatives {
		resp.Alternatives = append(resp.Alternatives, routeAlternativeToProto(alt))
	}

	for _, step := range r.Steps {
		pbStep := &LogiLens.RouteStep{
			From:     step.FromNodeID,
			To:       step.ToNodeID,
			Mode:     transportModeFromID(step.ModeID),
			Distance: step.Distance,
			Time:     step.Time,
			Cost:     step.Cost,
		}

		for _, point := range step.Geometry {
			pbStep.Geometry = append(pbStep.Geometry, &LogiLens.LatLng{
				Lat: point.Latitude,
				Lng: point.Longitude,
			})
		}

		resp.Steps = append(resp.Steps, pbStep)
	}

	// Flatten all step geometries into the top-level polyline.
	for _, pt := range r.Polyline {
		resp.Polyline = append(resp.Polyline, &LogiLens.LatLng{
			Lat: pt.Latitude,
			Lng: pt.Longitude,
		})
	}

	return resp
}

func routeAlternativeToProto(route comparison.Route) *LogiLens.RouteAlternative {
	alt := &LogiLens.RouteAlternative{
		RouteId:       route.RouteID,
		TotalDistance: route.TotalDistance,
		TotalTime:     route.TotalTime,
		TotalCost:     route.TotalCost,
	}

	for _, step := range route.Steps {
		pbStep := &LogiLens.RouteStep{
			From:     step.FromNodeID,
			To:       step.ToNodeID,
			Mode:     transportModeFromID(step.ModeID),
			Distance: step.Distance,
			Time:     step.Time,
			Cost:     step.Cost,
		}

		for _, point := range step.Geometry {
			pbStep.Geometry = append(pbStep.Geometry, &LogiLens.LatLng{
				Lat: point.Latitude,
				Lng: point.Longitude,
			})
		}

		alt.Steps = append(alt.Steps, pbStep)
	}

	// Flatten all step geometries into the top-level polyline.
	for _, pt := range route.Polyline {
		alt.Polyline = append(alt.Polyline, &LogiLens.LatLng{
			Lat: pt.Latitude,
			Lng: pt.Longitude,
		})
	}

	return alt
}

func transportModeFromID(modeID string) LogiLens.TransportMode {
	switch strings.ToLower(strings.TrimSpace(modeID)) {
	case "air", "air-transport", "plane":
		return LogiLens.TransportMode_AIR
	case "rail", "train":
		return LogiLens.TransportMode_RAIL
	case "ship", "sea", "marine":
		return LogiLens.TransportMode_SHIP
	default:
		return LogiLens.TransportMode_TRUCK
	}
}
