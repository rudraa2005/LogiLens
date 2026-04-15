package comparison

import (
	"sort"

	"github.com/rudraa2005/LogiLens/routing-service/models"
)

type Route struct {
	RouteID string `json:"route_id,omitempty"`

	SourceNodeID      string `json:"source_node_id,omitempty"`
	DestinationNodeID string `json:"destination_node_id,omitempty"`

	Steps []models.RouteStep `json:"steps"`

	Polyline []models.LatLng `json:"polyline"`

	TotalDistance float64 `json:"total_distance"`
	TotalTime     float64 `json:"total_time"`
	TotalCost     float64 `json:"total_cost"`
}

type BestRouteDecision struct {
	BestRoute    Route   `json:"best_route"`
	Alternatives []Route `json:"alternatives"`
	TimeSaved    float64 `json:"time_saved"`
	CostSaved    float64 `json:"cost_saved"`
}

func CompareRoutes(routes []Route) BestRouteDecision {
	if len(routes) == 0 {
		return BestRouteDecision{}
	}

	sorted := append([]Route(nil), routes...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return isRouteBetter(sorted[i], sorted[j])
	})

	decision := BestRouteDecision{
		BestRoute:    sorted[0],
		Alternatives: append([]Route(nil), sorted[1:]...),
	}

	if len(sorted) > 1 {
		decision.TimeSaved = sorted[1].TotalTime - sorted[0].TotalTime
		decision.CostSaved = sorted[1].TotalCost - sorted[0].TotalCost
	}

	return decision
}

func isRouteBetter(a, b Route) bool {
	switch {
	case a.TotalTime != b.TotalTime:
		return a.TotalTime < b.TotalTime
	case a.TotalCost != b.TotalCost:
		return a.TotalCost < b.TotalCost
	case a.TotalDistance != b.TotalDistance:
		return a.TotalDistance < b.TotalDistance
	default:
		return len(a.Steps) < len(b.Steps)
	}
}
