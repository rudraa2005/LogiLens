package graph

import (
	"github.com/rudraa2005/LogiLens/routing-service/models"
)

type Graph struct {
	Nodes     map[string]models.Node
	Adjacency map[string][]models.Edge
	NameToID  map[string]string
}

func BuildGraph(nodes []models.Node, edges []models.Edge) *Graph {

	graph := &Graph{
		Nodes:     make(map[string]models.Node),
		Adjacency: make(map[string][]models.Edge),
		NameToID:  make(map[string]string),
	}

	// 1. Load nodes
	for _, node := range nodes {
		graph.Nodes[node.NodeID] = node
		graph.NameToID[node.Name] = node.NodeID
	}

	// 2. Load edges into adjacency list
	for _, edge := range edges {
		graph.Adjacency[edge.From] = append(graph.Adjacency[edge.From], edge)

		ensureEdgeNode(graph, edge.From, edge.Geometry, true)
		ensureEdgeNode(graph, edge.To, edge.Geometry, false)
	}

	return graph
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
