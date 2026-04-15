package graph

import (
	"testing"
	"time"

	rctx "github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/models"
)

func TestKShortestPathsReturnsMultipleRoutes(t *testing.T) {
	g := BuildGraph([]models.Node{
		{NodeID: "s", Latitude: 0, Longitude: 0},
		{NodeID: "a", Latitude: 0, Longitude: 1},
		{NodeID: "b", Latitude: 1, Longitude: 0},
		{NodeID: "t", Latitude: 1, Longitude: 1},
	}, []models.Edge{
		{ID: "s-a", From: "s", To: "a", Distance: 1, Time: 1, Cost: 1},
		{ID: "a-t", From: "a", To: "t", Distance: 1, Time: 1, Cost: 1},
		{ID: "s-b", From: "s", To: "b", Distance: 1, Time: 2, Cost: 2},
		{ID: "b-t", From: "b", To: "t", Distance: 1, Time: 2, Cost: 2},
	})

	paths := g.KShortestPaths("s", "t", rctx.BuildContext(), "time", time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC), 5)
	if len(paths) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(paths))
	}

	if got := pathKey(paths[0].Nodes); got != "s->a->t" {
		t.Fatalf("expected fastest route first, got %q", got)
	}
	if paths[0].Score != 2 {
		t.Fatalf("unexpected first route score: %v", paths[0].Score)
	}
	if got := pathKey(paths[1].Nodes); got != "s->b->t" {
		t.Fatalf("expected second route second, got %q", got)
	}
	if paths[1].Score != 4 {
		t.Fatalf("unexpected second route score: %v", paths[1].Score)
	}
}
