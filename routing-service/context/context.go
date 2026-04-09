package context

import "github.com/rudraa2005/LogiLens/routing-service/models"

type EdgeContext struct {
	TrafficFactor float64
	WeatherFactor float64
	NewsFactor    float64
}

type Context struct {
	EdgeFactors map[string]EdgeContext
}

func BuildContext() Context {
	return Context{
		EdgeFactors: make(map[string]EdgeContext),
	}
}

func GetEdgeWeight(edge models.Edge, ctx Context, optimiseBy string) float64 {
	baseDistance := edge.Distance

	baseTime := edge.Time
	baseCost := edge.Cost

	factors, ok := ctx.EdgeFactors[edge.ID]

	traffic := 1.0
	weather := 1.0
	news := 1.0

	if ok {
		traffic = factors.TrafficFactor
		weather = factors.WeatherFactor
		news = factors.NewsFactor
	}

	time := baseTime * traffic * weather * news
	cost := baseCost * weather * news

	switch optimiseBy {
	case "time":
		return time
	case "cost":
		return cost
	case "distance":
		return baseDistance
	default:
		return time
	}
}
