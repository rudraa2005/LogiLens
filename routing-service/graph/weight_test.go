package graph

import (
	"testing"
	"time"

	rctx "github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/models"
)

func TestEdgeWeightUsesOptimizationModeAndContextFactors(t *testing.T) {
	edge := models.Edge{
		ID:       "edge-1",
		Distance: 7,
		Time:     10,
		Cost:     4,
	}
	ctx := rctx.Context{
		EdgeFactors: map[string]rctx.EdgeContext{
			"edge-1": {
				TrafficFactor: 1.2,
				WeatherFactor: 1.5,
				NewsFactor:    1.1,
			},
		},
	}

	if got := EdgeWeight(edge, ctx, "time", time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC)); got != 19.8 {
		t.Fatalf("unexpected time weight: %v", got)
	}
	if got := EdgeWeight(edge, ctx, "cost", time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC)); got != 7.92 {
		t.Fatalf("unexpected cost weight: %v", got)
	}
	if got := EdgeWeight(edge, ctx, "distance", time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC)); got != 7 {
		t.Fatalf("unexpected distance weight: %v", got)
	}
}
