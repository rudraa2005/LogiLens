package graph

import (
	"testing"

	"github.com/rudraa2005/LogiLens/routing-service/models"
)

func TestFindNearestNodeReturnsClosestByHaversine(t *testing.T) {
	g := BuildGraph([]models.Node{
		{NodeID: "delhi", Latitude: 28.6139, Longitude: 77.2090},
		{NodeID: "mumbai", Latitude: 19.0760, Longitude: 72.8777},
		{NodeID: "kolkata", Latitude: 22.5726, Longitude: 88.3639},
	}, nil)

	got := g.FindNearestNode(28.70, 77.10)
	if got != "delhi" {
		t.Fatalf("expected delhi, got %q", got)
	}
}

func TestFindNearestNodeReturnsEmptyForEmptyGraph(t *testing.T) {
	g := &Graph{}

	if got := g.FindNearestNode(0, 0); got != "" {
		t.Fatalf("expected empty result for empty graph, got %q", got)
	}
}
