package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	LogiLens "github.com/rudraa2005/LogiLens/proto"
	rctx "github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/geocoder"
	"github.com/rudraa2005/LogiLens/routing-service/graph"
	"github.com/rudraa2005/LogiLens/routing-service/models"
	"github.com/rudraa2005/LogiLens/routing-service/repository"
)

var geocodePlace = geocoder.Geocode

type RouteService struct {
	repo           routeRepository
	graph          *graph.Graph
	contextBuilder contextBuilder
}

func NewRouteService(repo *repository.RouteRepository, graph *graph.Graph) *RouteService {
	builder := rctx.NewContextService(graph.FindNearestEdge, func(lat, lng float64) string {
		nodeID := graph.FindNearestNode(lat, lng)
		if nodeID == "" {
			return ""
		}
		node := graph.Nodes[nodeID]
		if strings.TrimSpace(node.Name) != "" {
			return node.Name
		}
		return nodeID
	})

	return &RouteService{
		repo:           repo,
		graph:          graph,
		contextBuilder: builder,
	}
}

type routeRepository interface {
	CreateRoute(ctx context.Context, route models.Route) (string, error)
	CreateRouteSteps(ctx context.Context, routeID string, steps []models.RouteStep) error
}

type contextBuilder interface {
	Build(lat, lng float64) rctx.Context
}

type RouteRequest struct {
	Source      string
	Destination string
	OptimizeBy  string
}

type RouteResponse struct {
	Steps         []models.RouteStep
	RouteID       string
	TotalDistance float64
	TotalTime     float64
	TotalCost     float64
}

func (rs *RouteService) resolveNearestNode(place string) (string, float64, float64, error) {
	lat, lng, err := geocodePlace(place)
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
	sourceID, sourceLat, sourceLng, err := rs.resolveNearestNode(req.Source)
	if err != nil {
		return RouteResponse{}, err
	}

	destinationID, destLat, destLng, err := rs.resolveNearestNode(req.Destination)
	if err != nil {
		return RouteResponse{}, err
	}

	ctxData := rctx.BuildContext()
	if rs.contextBuilder != nil {
		midLat := (sourceLat + destLat) / 2
		midLng := (sourceLng + destLng) / 2
		ctxData = rs.contextBuilder.Build(midLat, midLng)
	}

	optimizeBy := req.OptimizeBy
	if optimizeBy == "" {
		optimizeBy = "time"
	}

	path, edges := rs.graph.Astar(sourceID, destinationID, ctxData, optimizeBy)
	if len(path) == 0 {
		return RouteResponse{}, errors.New("no route found")
	}

	steps := rs.BuildRoute(path, edges)

	var totalDistance, totalTime, totalCost float64
	for _, step := range steps {
		totalDistance += step.Distance
		totalCost += step.Cost
		totalTime += step.Time
	}

	route := models.Route{
		ID: uuid.New().String(),

		SourceNodeID:      sourceID,
		DestinationNodeID: destinationID,

		TotalDistance: totalDistance,
		TotalTime:     totalTime,
		TotalCost:     totalCost,
	}

	dbSteps := []models.RouteStep{}
	for i, step := range steps {
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

	routeID, err := rs.CreateRoute(ctx, route, dbSteps)
	if err != nil {
		return RouteResponse{}, err
	}

	return RouteResponse{
		RouteID:       routeID,
		Steps:         steps,
		TotalDistance: totalDistance,
		TotalTime:     totalTime,
		TotalCost:     totalCost,
	}, nil
}

func (r RouteResponse) ToProto() *LogiLens.RouteResponse {
	resp := &LogiLens.RouteResponse{
		RouteId:       r.RouteID,
		TotalDistance: r.TotalDistance,
		TotalTime:     r.TotalTime,
		TotalCost:     r.TotalCost,
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

	return resp
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
