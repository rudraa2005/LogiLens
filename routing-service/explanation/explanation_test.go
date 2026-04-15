package explanation

import (
	"strings"
	"testing"

	"github.com/rudraa2005/LogiLens/routing-service/comparison"
	rctx "github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/models"
)

func TestExplainRouteSummarizesAvoidedIssuesAndSavings(t *testing.T) {
	best := comparison.Route{
		RouteID: "best",
		Steps: []models.RouteStep{
			{EdgeID: "edge-best-1", FromNodeID: "A", ToNodeID: "B"},
			{EdgeID: "edge-best-2", FromNodeID: "B", ToNodeID: "C"},
		},
		TotalTime: 42,
		TotalCost: 15,
	}
	alternatives := []comparison.Route{
		{
			RouteID: "alt-1",
			Steps: []models.RouteStep{
				{EdgeID: "edge-alt-1", FromNodeID: "A", ToNodeID: "X"},
				{EdgeID: "edge-alt-2", FromNodeID: "X", ToNodeID: "C"},
			},
			TotalTime: 60,
			TotalCost: 20,
		},
		{
			RouteID: "alt-2",
			Steps: []models.RouteStep{
				{EdgeID: "edge-alt-3", FromNodeID: "A", ToNodeID: "Y"},
				{EdgeID: "edge-alt-4", FromNodeID: "Y", ToNodeID: "C"},
			},
			TotalTime: 55,
			TotalCost: 18,
		},
	}
	ctx := rctx.Context{
		BaseEdgeFactors: map[string]rctx.EdgeContext{
			"edge-alt-1": {TrafficFactor: 1.9},
			"edge-alt-3": {WeatherFactor: 1.8},
		},
	}

	got := ExplainRoute(best, alternatives, ctx)

	if !strings.Contains(got, "heavy congestion near X") {
		t.Fatalf("expected congestion explanation, got %q", got)
	}
	if !strings.Contains(got, "severe weather near Y") {
		t.Fatalf("expected weather explanation, got %q", got)
	}
	if !strings.Contains(got, "saving 13 minutes") {
		t.Fatalf("expected time savings against fastest alternative, got %q", got)
	}
	if !strings.HasSuffix(got, ".") {
		t.Fatalf("expected trailing period, got %q", got)
	}
}

func TestExplainRouteFallsBackWhenNoSignalsExist(t *testing.T) {
	got := ExplainRoute(comparison.Route{RouteID: "best"}, nil, rctx.BuildContext())
	if !strings.Contains(got, "best available option") {
		t.Fatalf("expected fallback explanation, got %q", got)
	}
}
