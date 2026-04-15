package comparison

import (
	"testing"

	"github.com/rudraa2005/LogiLens/routing-service/models"
)

func TestCompareRoutesRanksByTimeThenCostThenDistance(t *testing.T) {
	routes := []Route{
		{RouteID: "r2", TotalTime: 12, TotalCost: 9, TotalDistance: 5},
		{RouteID: "r1", TotalTime: 10, TotalCost: 12, TotalDistance: 7},
		{RouteID: "r3", TotalTime: 10, TotalCost: 8, TotalDistance: 9},
	}

	decision := CompareRoutes(routes)
	if decision.BestRoute.RouteID != "r3" {
		t.Fatalf("expected r3 to win tie-break on cost, got %q", decision.BestRoute.RouteID)
	}
	if len(decision.Alternatives) != 2 {
		t.Fatalf("expected 2 alternatives, got %d", len(decision.Alternatives))
	}
	if decision.TimeSaved != 0 {
		t.Fatalf("expected zero time saved against runner-up with same time, got %v", decision.TimeSaved)
	}
	if decision.CostSaved != 4 {
		t.Fatalf("expected 4 cost saved against runner-up, got %v", decision.CostSaved)
	}
}

func TestCompareRoutesReturnsEmptyDecisionForEmptyInput(t *testing.T) {
	decision := CompareRoutes(nil)
	if decision.BestRoute.RouteID != "" {
		t.Fatalf("expected empty best route, got %q", decision.BestRoute.RouteID)
	}
	if len(decision.Alternatives) != 0 {
		t.Fatalf("expected no alternatives, got %d", len(decision.Alternatives))
	}
}

func TestCompareRoutesKeepsStepsIntact(t *testing.T) {
	step := models.RouteStep{ID: "step-1"}
	decision := CompareRoutes([]Route{
		{RouteID: "r1", Steps: []models.RouteStep{step}, TotalTime: 1, TotalCost: 1, TotalDistance: 1},
	})

	if len(decision.BestRoute.Steps) != 1 || decision.BestRoute.Steps[0].ID != "step-1" {
		t.Fatalf("expected steps to be preserved, got %#v", decision.BestRoute.Steps)
	}
}
