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
	baseTime := edge.Time
	baseCost := edge.Cost
	baseDistance := edge.Distance

	factors := ctx.EdgeContextAt(edge.ID, eta)
	traffic := clampFactor(factors.TrafficFactor)
	weather := clampFactor(factors.WeatherFactor)
	news := clampFactor(factors.NewsFactor)
	aiFactor := clampAIFactor(factors.AIFactor)

	switch strings.ToLower(strings.TrimSpace(optimizeBy)) {
	case "cost":
		return baseCost * traffic * weather * news * aiFactor
	case "distance":
		return baseDistance
	default:
		return baseTime * traffic * weather * news * aiFactor
	}
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
