package graph

import (
	"sort"

	"github.com/rudraa2005/LogiLens/routing-service/models"
)

const heuristicSafetyMultiplier = 1.1

type Graph struct {
	Nodes     map[string]models.Node
	Adjacency map[string][]models.Edge
	NameToID  map[string]string
	Edges     map[string]models.Edge

	heuristicSpeedKPH float64
	edgeNeighbors     map[string][]string
	spatial           *SpatialIndex
}

func BuildGraph(nodes []models.Node, edges []models.Edge) *Graph {

	graph := &Graph{
		Nodes:     make(map[string]models.Node),
		Adjacency: make(map[string][]models.Edge),
		NameToID:  make(map[string]string),
		Edges:     make(map[string]models.Edge),
	}

	// 1. Load nodes
	for _, node := range nodes {
		graph.Nodes[node.NodeID] = node
		graph.NameToID[node.Name] = node.NodeID
	}

	// 2. Load edges into adjacency list
	for _, edge := range edges {
		graph.Adjacency[edge.From] = append(graph.Adjacency[edge.From], edge)
		if edge.ID != "" {
			graph.Edges[edge.ID] = edge
		}
		updateGraphMetrics(graph, edge)

		ensureEdgeNode(graph, edge.From, edge.Geometry, true)
		ensureEdgeNode(graph, edge.To, edge.Geometry, false)
	}

	graph.buildEdgeNeighbors()
	graph.spatial = BuildSpatialIndex(graph.Nodes, edges)

	return graph
}

func updateGraphMetrics(g *Graph, edge models.Edge) {
	if g == nil {
		return
	}
	if edge.Distance <= 0 || edge.Time <= 0 {
		return
	}

	speedKPH := edge.Distance / (edge.Time / 60.0)
	heuristicSpeedKPH := speedKPH / 0.8 * heuristicSafetyMultiplier
	if heuristicSpeedKPH > g.heuristicSpeedKPH {
		g.heuristicSpeedKPH = heuristicSpeedKPH
	}
}

func (g *Graph) buildEdgeNeighbors() {
	if g == nil {
		return
	}

	nodeEdges := make(map[string]map[string]struct{})
	for _, edge := range g.Edges {
		if edge.From != "" {
			if _, ok := nodeEdges[edge.From]; !ok {
				nodeEdges[edge.From] = make(map[string]struct{})
			}
			nodeEdges[edge.From][edge.ID] = struct{}{}
		}
		if edge.To != "" {
			if _, ok := nodeEdges[edge.To]; !ok {
				nodeEdges[edge.To] = make(map[string]struct{})
			}
			nodeEdges[edge.To][edge.ID] = struct{}{}
		}
	}

	g.edgeNeighbors = make(map[string][]string, len(g.Edges))
	for _, edge := range g.Edges {
		neighbors := make(map[string]struct{})
		for _, nodeID := range []string{edge.From, edge.To} {
			for candidate := range nodeEdges[nodeID] {
				if candidate == "" || candidate == edge.ID {
					continue
				}
				neighbors[candidate] = struct{}{}
			}
		}
		g.edgeNeighbors[edge.ID] = keysFromSet(neighbors)
	}
}

func (g *Graph) GetNeighborEdges(edgeID string) []string {
	if g == nil || edgeID == "" {
		return nil
	}
	neighbors := g.edgeNeighbors[edgeID]
	out := make([]string, 0, len(neighbors))
	out = append(out, neighbors...)
	return out
}

func keysFromSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func ensureEdgeNode(g *Graph, nodeID string, geometry []models.LatLng, useFirstPoint bool) {
	if nodeID == "" {
		return
	}

	if _, exists := g.Nodes[nodeID]; exists {
		return
	}

	node := models.Node{
		NodeID: nodeID,
		Name:   nodeID,
	}

	if len(geometry) > 0 {
		point := geometry[0]
		if !useFirstPoint {
			point = geometry[len(geometry)-1]
		}
		node.Latitude = point.Latitude
		node.Longitude = point.Longitude
	}

	g.Nodes[nodeID] = node
	if node.Name != "" {
		g.NameToID[node.Name] = nodeID
	}
}
