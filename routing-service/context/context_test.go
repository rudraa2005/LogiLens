package context

import (
	"testing"
	"time"
)

func TestEdgeContextAtUsesTimeBuckets(t *testing.T) {
	ctx := BuildContext()
	ctx.BaseEdgeFactors["edge-1"] = EdgeContext{TrafficFactor: 1.1}
	ctx.AddTimedEdgeFactor("edge-1", time.Date(2026, 4, 15, 8, 7, 0, 0, time.UTC), EdgeContext{TrafficFactor: 1.4})
	ctx.AddTimedEdgeFactor("edge-1", time.Date(2026, 4, 15, 8, 29, 0, 0, time.UTC), EdgeContext{TrafficFactor: 1.8})

	if got := ctx.EdgeContextAt("edge-1", time.Date(2026, 4, 15, 8, 14, 0, 0, time.UTC)).TrafficFactor; got != 1.4 {
		t.Fatalf("expected 08:00 bucket factor 1.4, got %v", got)
	}
	if got := ctx.EdgeContextAt("edge-1", time.Date(2026, 4, 15, 8, 29, 0, 0, time.UTC)).TrafficFactor; got != 1.8 {
		t.Fatalf("expected 08:15 bucket factor 1.8, got %v", got)
	}
	if got := ctx.EdgeContextAt("edge-1", time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC)).TrafficFactor; got != 1.1 {
		t.Fatalf("expected fallback base factor 1.1, got %v", got)
	}
}

func TestEdgeContextAtReturnsNeutralFallback(t *testing.T) {
	ctx := BuildContext()
	got := ctx.EdgeContextAt("missing-edge", time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC))
	if got.TrafficFactor != 1.0 || got.WeatherFactor != 1.0 || got.NewsFactor != 1.0 || got.AIFactor != 1.0 {
		t.Fatalf("expected neutral fallback, got %+v", got)
	}
}
