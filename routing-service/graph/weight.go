package graph

import (
	"strings"

	"github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/models"
)

func EdgeWeight(edge models.Edge, ctx context.Context, optimizeBy string) float64 {
	baseTime := edge.Time
	baseCost := edge.Cost
	baseDistance := edge.Distance

	factors, ok := ctx.EdgeFactors[edge.ID]

	traffic := 1.0
	weather := 1.0
	news := 1.0
	if ok {
		traffic = factors.TrafficFactor
		weather = factors.WeatherFactor
		news = factors.NewsFactor
	}

	switch strings.ToLower(strings.TrimSpace(optimizeBy)) {
	case "cost":
		return baseCost * traffic * weather * news
	case "distance":
		return baseDistance
	default:
		return baseTime * traffic * weather * news
	}
}
