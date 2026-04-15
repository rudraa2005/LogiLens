package graph

import (
	"math"
	"sort"

	"github.com/rudraa2005/LogiLens/routing-service/models"
)

const (
	defaultSpatialCellSizeKm  = 1.0
	defaultEdgeSampleStepKm   = 0.2
	maxNearestNodeSearchRings = 24
)

type cellKey struct {
	lat int
	lng int
}

type EdgeDistance struct {
	EdgeID     string
	DistanceKm float64
}

type SpatialIndex struct {
	cellSizeKm  float64
	cellSizeDeg float64

	nodeBuckets map[cellKey][]string
	edgeBuckets map[cellKey][]string
}

func BuildSpatialIndex(nodes map[string]models.Node, edges []models.Edge) *SpatialIndex {
	index := &SpatialIndex{
		cellSizeKm:  defaultSpatialCellSizeKm,
		cellSizeDeg: defaultSpatialCellSizeKm / 111.0,
		nodeBuckets: make(map[cellKey][]string),
		edgeBuckets: make(map[cellKey][]string),
	}

	for nodeID, node := range nodes {
		index.nodeBuckets[index.cellFor(node.Latitude, node.Longitude)] = append(
			index.nodeBuckets[index.cellFor(node.Latitude, node.Longitude)],
			nodeID,
		)
	}

	for _, edge := range edges {
		if edge.ID == "" {
			continue
		}
		for _, point := range sampledGeometry(edge.Geometry) {
			key := index.cellFor(point.Latitude, point.Longitude)
			index.edgeBuckets[key] = append(index.edgeBuckets[key], edge.ID)
		}
	}

	return index
}

func (s *SpatialIndex) cellFor(lat, lng float64) cellKey {
	if s == nil || s.cellSizeDeg <= 0 {
		return cellKey{}
	}
	return cellKey{
		lat: int(math.Floor(lat / s.cellSizeDeg)),
		lng: int(math.Floor(lng / s.cellSizeDeg)),
	}
}

func (s *SpatialIndex) nodeCandidates(lat, lng float64, rings int) []string {
	if s == nil {
		return nil
	}
	center := s.cellFor(lat, lng)
	candidates := make([]string, 0)
	for dy := -rings; dy <= rings; dy++ {
		for dx := -rings; dx <= rings; dx++ {
			key := cellKey{lat: center.lat + dy, lng: center.lng + dx}
			candidates = append(candidates, s.nodeBuckets[key]...)
		}
	}
	return candidates
}

func (s *SpatialIndex) edgeCandidates(lat, lng, radiusKm float64) []string {
	if s == nil {
		return nil
	}
	rings := int(math.Ceil(radiusKm / s.cellSizeKm))
	center := s.cellFor(lat, lng)
	seen := make(map[string]struct{})
	candidates := make([]string, 0)
	for dy := -rings; dy <= rings; dy++ {
		for dx := -rings; dx <= rings; dx++ {
			key := cellKey{lat: center.lat + dy, lng: center.lng + dx}
			for _, edgeID := range s.edgeBuckets[key] {
				if _, ok := seen[edgeID]; ok {
					continue
				}
				seen[edgeID] = struct{}{}
				candidates = append(candidates, edgeID)
			}
		}
	}
	return candidates
}

func sampledGeometry(points []models.LatLng) []models.LatLng {
	if len(points) == 0 {
		return nil
	}
	if len(points) == 1 {
		return append([]models.LatLng(nil), points[0])
	}

	samples := make([]models.LatLng, 0, len(points))
	for i := 0; i < len(points)-1; i++ {
		start := points[i]
		end := points[i+1]
		samples = append(samples, start)

		segmentKm := haversineDistanceKm(start.Latitude, start.Longitude, end.Latitude, end.Longitude)
		if segmentKm <= defaultEdgeSampleStepKm {
			continue
		}

		steps := int(math.Ceil(segmentKm / defaultEdgeSampleStepKm))
		for step := 1; step < steps; step++ {
			t := float64(step) / float64(steps)
			samples = append(samples, models.LatLng{
				Latitude:  start.Latitude + (end.Latitude-start.Latitude)*t,
				Longitude: start.Longitude + (end.Longitude-start.Longitude)*t,
			})
		}
	}
	samples = append(samples, points[len(points)-1])
	return samples
}

func sortEdgeDistances(matches []EdgeDistance) {
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].DistanceKm != matches[j].DistanceKm {
			return matches[i].DistanceKm < matches[j].DistanceKm
		}
		return matches[i].EdgeID < matches[j].EdgeID
	})
}
