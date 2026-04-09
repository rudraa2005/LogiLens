package graph

import "math"

const earthRadiusKm = 6371.0

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
