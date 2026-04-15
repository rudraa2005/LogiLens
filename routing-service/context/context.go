package context

import "time"

type EdgeContext struct {
	TrafficFactor float64
	WeatherFactor float64
	NewsFactor    float64
	AIFactor      float64
}

type TimedEdgeContext struct {
	EffectiveAt time.Time
	Factors     EdgeContext
}

type Context struct {
	LocationName     string
	DepartureTime    time.Time
	EdgeFactors      map[string]EdgeContext
	EdgeArrivalTimes map[string]time.Time
	EdgeTimeFactors  map[string][]TimedEdgeContext
}

func BuildContext() Context {
	return Context{
		EdgeFactors:      make(map[string]EdgeContext),
		EdgeArrivalTimes: make(map[string]time.Time),
		EdgeTimeFactors:  make(map[string][]TimedEdgeContext),
	}
}

func (c Context) EdgeContextAt(edgeID string, eta time.Time) EdgeContext {
	base := c.EdgeFactors[edgeID]
	entries := c.EdgeTimeFactors[edgeID]
	if len(entries) == 0 || eta.IsZero() {
		return normalizeEdgeContext(base)
	}

	selected := entries[0]
	foundPast := false
	for _, entry := range entries {
		if entry.EffectiveAt.IsZero() {
			continue
		}
		if !entry.EffectiveAt.After(eta) {
			if !foundPast || entry.EffectiveAt.After(selected.EffectiveAt) {
				selected = entry
				foundPast = true
			}
			continue
		}

		if foundPast {
			continue
		}
		if selected.EffectiveAt.IsZero() || entry.EffectiveAt.Before(selected.EffectiveAt) {
			selected = entry
		}
	}

	return normalizeEdgeContext(mergeEdgeContext(base, selected.Factors))
}

func (c *Context) AddTimedEdgeFactor(edgeID string, eta time.Time, delta EdgeContext) {
	if c == nil || edgeID == "" || eta.IsZero() {
		return
	}

	hour := eta.UTC().Truncate(time.Hour)
	existing := c.EdgeTimeFactors[edgeID]
	for i := range existing {
		if existing[i].EffectiveAt.Equal(hour) {
			existing[i].Factors = mergeEdgeContext(existing[i].Factors, delta)
			c.EdgeTimeFactors[edgeID] = existing
			return
		}
	}

	c.EdgeTimeFactors[edgeID] = append(c.EdgeTimeFactors[edgeID], TimedEdgeContext{
		EffectiveAt: hour,
		Factors:     delta,
	})
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
