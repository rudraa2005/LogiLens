package graph

import (
	"math"

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

	if g.spatial != nil {
		nearestID := ""
		minDistance := math.Inf(1)

		for ring := 0; ring <= maxNearestNodeSearchRings; ring++ {
			candidates := g.spatial.nodeCandidates(lat, lng, ring)
			for _, nodeID := range candidates {
				node, ok := g.Nodes[nodeID]
				if !ok {
					continue
				}
				distance := haversineDistanceKm(lat, lng, node.Latitude, node.Longitude)
				if distance < minDistance {
					minDistance = distance
					nearestID = nodeID
				}
			}

			if nearestID != "" {
				// Once the next ring cannot contain a closer node than the
				// current best, we can stop expanding.
				minPossible := math.Max(0, float64(ring)*g.spatial.cellSizeKm-math.Sqrt2*g.spatial.cellSizeKm)
				if minPossible >= minDistance {
					return nearestID
				}
			}
		}

		if nearestID != "" {
			return nearestID
		}
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

	for _, radiusKm := range []float64{nearbyEdgeThresholdKm, 1, 3, 10} {
		matches := g.FindNearbyEdgeDistances(lat, lng, radiusKm)
		if len(matches) > 0 {
			return matches[0].EdgeID
		}
	}

	nearestID := ""
	minDistance := math.Inf(1)
	for _, edge := range g.Edges {
		distance := edgeGeometryDistanceKm(g, edge, lat, lng)
		if distance < minDistance {
			minDistance = distance
			nearestID = edge.ID
		}
	}
	return nearestID
}

// FindNearbyEdges returns all edge IDs whose geometry lies within a fixed
// distance threshold of the provided point.
func (g *Graph) FindNearbyEdges(lat, lng float64, radiusKm ...float64) []string {
	matches := g.FindNearbyEdgeDistances(lat, lng, radiusKm...)
	edgeIDs := make([]string, 0, len(matches))
	for _, match := range matches {
		edgeIDs = append(edgeIDs, match.EdgeID)
	}
	return edgeIDs
}

func (g *Graph) FindNearbyEdgeDistances(lat, lng float64, radiusKm ...float64) []EdgeDistance {
	if g == nil || len(g.Adjacency) == 0 {
		return nil
	}

	thresholdKm := nearbyEdgeThresholdKm
	if len(radiusKm) > 0 && radiusKm[0] > 0 {
		thresholdKm = radiusKm[0]
	}

	candidateIDs := make([]string, 0)
	if g.spatial != nil {
		candidateIDs = g.spatial.edgeCandidates(lat, lng, thresholdKm)
	}
	if len(candidateIDs) == 0 {
		for edgeID := range g.Edges {
			candidateIDs = append(candidateIDs, edgeID)
		}
	}

	matches := make([]EdgeDistance, 0)
	seen := make(map[string]struct{})
	for _, edgeID := range candidateIDs {
		if _, ok := seen[edgeID]; ok {
			continue
		}
		seen[edgeID] = struct{}{}

		edge, ok := g.Edges[edgeID]
		if !ok {
			continue
		}

		distance := edgeGeometryDistanceKm(g, edge, lat, lng)
		if distance > thresholdKm {
			continue
		}
		matches = append(matches, EdgeDistance{
			EdgeID:     edgeID,
			DistanceKm: distance,
		})
	}

	sortEdgeDistances(matches)
	return matches
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
