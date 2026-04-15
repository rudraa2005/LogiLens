package graph

import (
	"testing"

	"github.com/rudraa2005/LogiLens/routing-service/models"
)

func TestFindNearbyEdgesReturnsMultipleEdgesWithinThreshold(t *testing.T) {
	g := BuildGraph([]models.Node{
		{NodeID: "n1", Latitude: 0, Longitude: 0},
		{NodeID: "n2", Latitude: 0, Longitude: 1},
	}, []models.Edge{
		{
			ID:   "edge-a",
			From: "n1",
			To:   "n2",
			Geometry: []models.LatLng{
				{Latitude: 0.0005, Longitude: -0.001},
				{Latitude: 0.0005, Longitude: 0.001},
			},
		},
		{
			ID:   "edge-b",
			From: "n1",
			To:   "n2",
			Geometry: []models.LatLng{
				{Latitude: -0.0005, Longitude: -0.001},
				{Latitude: -0.0005, Longitude: 0.001},
			},
		},
		{
			ID:   "edge-far",
			From: "n1",
			To:   "n2",
			Geometry: []models.LatLng{
				{Latitude: 1, Longitude: 1},
				{Latitude: 1, Longitude: 1.1},
			},
		},
	})

	got := g.FindNearbyEdges(0, 0)
	if len(got) != 2 {
		t.Fatalf("expected 2 nearby edges, got %d (%v)", len(got), got)
	}
	if got[0] != "edge-a" || got[1] != "edge-b" {
		t.Fatalf("unexpected nearby edges order: %v", got)
	}
}
