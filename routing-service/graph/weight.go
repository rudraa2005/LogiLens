package graph

import (
	"strings"
	"time"

	rctx "github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/models"
)

const (
	minContextFactor = 1.0
	maxContextFactor = 2.5
)

func EdgeWeight(edge models.Edge, ctx rctx.Context, optimizeBy string, eta time.Time) float64 {
	switch strings.ToLower(strings.TrimSpace(optimizeBy)) {
	case "cost":
		return CostWeight(edge, ctx, eta)
	case "distance":
		return edge.Distance
	default:
		return TravelTime(edge, ctx, eta)
	}
}

func CombinedFactor(edge models.Edge, ctx rctx.Context, eta time.Time) float64 {
	factors := ctx.EdgeContextAt(edge.ID, eta)
	traffic := clampFactor(factors.TrafficFactor)
	weather := clampFactor(factors.WeatherFactor)
	news := clampFactor(factors.NewsFactor)
	aiFactor := clampAIFactor(factors.AIFactor)
	return traffic * weather * news * aiFactor
}

func TravelTime(edge models.Edge, ctx rctx.Context, eta time.Time) float64 {
	return edge.Time * CombinedFactor(edge, ctx, eta)
}

func CostWeight(edge models.Edge, ctx rctx.Context, eta time.Time) float64 {
	return edge.Cost * CombinedFactor(edge, ctx, eta)
}

func clampAIFactor(value float64) float64 {
	switch {
	case value <= 0:
		return 1.0
	case value < 0.8:
		return 0.8
	case value > 1.5:
		return 1.5
	default:
		return value
	}
}

func clampFactor(value float64) float64 {
	switch {
	case value <= 0:
		return minContextFactor
	case value < minContextFactor:
		return minContextFactor
	case value > maxContextFactor:
		return maxContextFactor
	default:
		return value
	}
}
