package graph

import (
	"testing"
	"time"

	rctx "github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/models"
)

func TestAstarChangesRouteWhenETAShiftsBuckets(t *testing.T) {
	g := BuildGraph([]models.Node{
		{NodeID: "s", Latitude: 0, Longitude: 0},
		{NodeID: "a", Latitude: 0, Longitude: 0.1},
		{NodeID: "b", Latitude: 0.1, Longitude: 0},
		{NodeID: "t", Latitude: 0.1, Longitude: 0.1},
	}, []models.Edge{
		{ID: "s-a", From: "s", To: "a", Distance: 5, Time: 10, Cost: 5},
		{ID: "a-t", From: "a", To: "t", Distance: 5, Time: 10, Cost: 5},
		{ID: "s-b", From: "s", To: "b", Distance: 5, Time: 12, Cost: 5},
		{ID: "b-t", From: "b", To: "t", Distance: 5, Time: 12, Cost: 5},
	})

	ctx := rctx.BuildContext()
	ctx.AddTimedEdgeFactor("a-t", time.Date(2026, 4, 15, 8, 15, 0, 0, time.UTC), rctx.EdgeContext{TrafficFactor: 2.0})

	earlyPath, _ := g.Astar("s", "t", ctx, "time", time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC))
	latePath, _ := g.Astar("s", "t", ctx, "time", time.Date(2026, 4, 15, 8, 10, 0, 0, time.UTC))

	if got := pathKey(earlyPath); got != "s->a->t" {
		t.Fatalf("expected early departure to use route A, got %q", got)
	}
	if got := pathKey(latePath); got != "s->b->t" {
		t.Fatalf("expected later departure to avoid delayed ETA bucket, got %q", got)
	}
}

func TestTimeHeuristicDoesNotOverestimateFastestPossibleTravel(t *testing.T) {
	g := BuildGraph([]models.Node{
		{NodeID: "s", Latitude: 0, Longitude: 0},
		{NodeID: "t", Latitude: 0, Longitude: 0.1},
	}, []models.Edge{
		{ID: "s-t", From: "s", To: "t", Distance: 11.1, Time: 10, Cost: 4},
	})

	ctx := rctx.BuildContext()
	ctx.BaseEdgeFactors["s-t"] = rctx.EdgeContext{AIFactor: 0.8}
	travelTime := TravelTime(g.Edges["s-t"], ctx, time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC))
	heuristic := g.heuristic("s", "t", "time")

	if heuristic > travelTime {
		t.Fatalf("expected admissible heuristic, got heuristic=%v travelTime=%v", heuristic, travelTime)
	}
}
