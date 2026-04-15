package graph

import (
	"math"
	"sort"

	"github.com/rudraa2005/LogiLens/routing-service/models"
)

const earthRadiusKm = 6371.0
const nearbyEdgeThresholdKm = 0.35

// FindNearestNode returns the node ID whose coordinates are closest to the
// provided latitude and longitude using the Haversine formula.
func (g *Graph) FindNearestNode(lat, lng float64) string {
	if g == nil || len(g.Nodes) == 0 {
		return ""
	}

	nearestID := ""
	minDistance := math.Inf(1)

	for nodeID, node := range g.Nodes {
		distance := haversineDistanceKm(lat, lng, node.Latitude, node.Longitude)
		if distance < minDistance {
			minDistance = distance
			nearestID = nodeID
		}
	}

	return nearestID
}

// FindNearestEdge returns the edge ID whose geometry contains the point closest
// to the provided latitude and longitude using the Haversine formula.
func (g *Graph) FindNearestEdge(lat, lng float64) string {
	if g == nil || len(g.Adjacency) == 0 {
		return ""
	}

	nearestID := ""
	minDistance := math.Inf(1)

	for _, edges := range g.Adjacency {
		for _, edge := range edges {
			for _, point := range edge.Geometry {
				distance := haversineDistanceKm(lat, lng, point.Latitude, point.Longitude)
				if distance < minDistance {
					minDistance = distance
					nearestID = edge.ID
				}
			}
		}
	}

	return nearestID
}

// FindNearbyEdges returns all edge IDs whose geometry lies within a fixed
// distance threshold of the provided point.
func (g *Graph) FindNearbyEdges(lat, lng float64) []string {
	if g == nil || len(g.Adjacency) == 0 {
		return nil
	}

	edgeIDs := make([]string, 0)
	seen := make(map[string]struct{})

	for _, edges := range g.Adjacency {
		for _, edge := range edges {
			if edge.ID == "" {
				continue
			}

			distance := edgeGeometryDistanceKm(g, edge, lat, lng)
			if distance > nearbyEdgeThresholdKm {
				continue
			}

			if _, ok := seen[edge.ID]; ok {
				continue
			}

			seen[edge.ID] = struct{}{}
			edgeIDs = append(edgeIDs, edge.ID)
		}
	}

	sort.Strings(edgeIDs)
	return edgeIDs
}

func haversineDistanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	dLat := degreesToRadians(lat2 - lat1)
	dLon := degreesToRadians(lon2 - lon1)

	lat1Rad := degreesToRadians(lat1)
	lat2Rad := degreesToRadians(lat2)

	sinLat := math.Sin(dLat / 2)
	sinLon := math.Sin(dLon / 2)

	a := sinLat*sinLat + math.Cos(lat1Rad)*math.Cos(lat2Rad)*sinLon*sinLon
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180.0
}

func edgeGeometryDistanceKm(g *Graph, edge models.Edge, lat, lng float64) float64 {
	points := edge.Geometry
	if len(points) == 0 {
		from, okFrom := g.Nodes[edge.From]
		to, okTo := g.Nodes[edge.To]
		if !okFrom || !okTo {
			return math.Inf(1)
		}
		points = []models.LatLng{
			{Latitude: from.Latitude, Longitude: from.Longitude},
			{Latitude: to.Latitude, Longitude: to.Longitude},
		}
	}

	if len(points) == 1 {
		return haversineDistanceKm(lat, lng, points[0].Latitude, points[0].Longitude)
	}

	minDistance := math.Inf(1)
	for i := 0; i < len(points)-1; i++ {
		distance := pointToSegmentDistanceKm(lat, lng, points[i], points[i+1])
		if distance < minDistance {
			minDistance = distance
		}
	}

	return minDistance
}

func pointToSegmentDistanceKm(lat, lng float64, a, b models.LatLng) float64 {
	ax, ay := projectToLocalKm(lat, lng, a.Latitude, a.Longitude)
	bx, by := projectToLocalKm(lat, lng, b.Latitude, b.Longitude)

	dx := bx - ax
	dy := by - ay
	denom := dx*dx + dy*dy
	if denom == 0 {
		return math.Hypot(ax, ay)
	}

	t := -(ax*dx + ay*dy) / denom
	switch {
	case t < 0:
		t = 0
	case t > 1:
		t = 1
	}

	closestX := ax + t*dx
	closestY := ay + t*dy
	return math.Hypot(closestX, closestY)
}

func projectToLocalKm(originLat, originLng, lat, lng float64) (float64, float64) {
	latKm := degreesToRadians(lat-originLat) * earthRadiusKm
	lonScale := math.Cos(degreesToRadians(originLat))
	lonKm := degreesToRadians(lng-originLng) * earthRadiusKm * lonScale
	return lonKm, latKm
}
