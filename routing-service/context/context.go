package context

import "time"

const TimeBucketSize = 15 * time.Minute

type EdgeContext struct {
	TrafficFactor float64
	WeatherFactor float64
	NewsFactor    float64
	AIFactor      float64
}

type Context struct {
	LocationName     string
	DepartureTime    time.Time
	BaseEdgeFactors  map[string]EdgeContext
	EdgeFactors      map[string]map[time.Time]EdgeContext
	EdgeArrivalTimes map[string]time.Time
}

func BuildContext() Context {
	return Context{
		BaseEdgeFactors:  make(map[string]EdgeContext),
		EdgeFactors:      make(map[string]map[time.Time]EdgeContext),
		EdgeArrivalTimes: make(map[string]time.Time),
	}
}

func (c Context) EdgeContextAt(edgeID string, eta time.Time) EdgeContext {
	base := c.BaseEdgeFactors[edgeID]
	if eta.IsZero() {
		return normalizeEdgeContext(base)
	}

	bucket := TimeBucket(eta)
	buckets, ok := c.EdgeFactors[edgeID]
	if !ok {
		return normalizeEdgeContext(base)
	}
	return normalizeEdgeContext(mergeEdgeContext(base, buckets[bucket]))
}

func (c *Context) AddTimedEdgeFactor(edgeID string, eta time.Time, delta EdgeContext) {
	if c == nil || edgeID == "" || eta.IsZero() {
		return
	}

	if c.EdgeFactors == nil {
		c.EdgeFactors = make(map[string]map[time.Time]EdgeContext)
	}
	bucket := TimeBucket(eta)
	if _, ok := c.EdgeFactors[edgeID]; !ok {
		c.EdgeFactors[edgeID] = make(map[time.Time]EdgeContext)
	}
	c.EdgeFactors[edgeID][bucket] = mergeEdgeContext(c.EdgeFactors[edgeID][bucket], delta)
}

func mergeEdgeContext(existing, delta EdgeContext) EdgeContext {
	if delta.TrafficFactor > existing.TrafficFactor {
		existing.TrafficFactor = delta.TrafficFactor
	}
	if delta.WeatherFactor > existing.WeatherFactor {
		existing.WeatherFactor = delta.WeatherFactor
	}
	if delta.NewsFactor > existing.NewsFactor {
		existing.NewsFactor = delta.NewsFactor
	}
	if delta.AIFactor > 0 {
		existing.AIFactor = delta.AIFactor
	}
	return existing
}

func TimeBucket(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value.UTC().Truncate(TimeBucketSize)
}

func normalizeEdgeContext(value EdgeContext) EdgeContext {
	if value.TrafficFactor <= 0 {
		value.TrafficFactor = 1.0
	}
	if value.WeatherFactor <= 0 {
		value.WeatherFactor = 1.0
	}
	if value.NewsFactor <= 0 {
		value.NewsFactor = 1.0
	}
	if value.AIFactor <= 0 {
		value.AIFactor = 1.0
	}
	return value
}
