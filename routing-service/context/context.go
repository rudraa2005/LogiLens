package context

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
