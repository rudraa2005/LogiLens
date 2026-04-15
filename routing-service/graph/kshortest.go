package graph

import (
	"container/heap"
	"strings"

	rctx "github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/models"
)

type SearchConstraints struct {
	BlockedNodes map[string]struct{}
	BlockedEdges map[string]struct{}
}

type PathResult struct {
	Nodes []string
	Edges map[string]models.Edge
	Score float64
}

type pathCandidate struct {
	PathResult
}

type pathCandidateHeap []pathCandidate

func (h pathCandidateHeap) Len() int { return len(h) }

func (h pathCandidateHeap) Less(i, j int) bool { return h[i].Score < h[j].Score }

func (h pathCandidateHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *pathCandidateHeap) Push(x interface{}) {
	*h = append(*h, x.(pathCandidate))
}

func (h *pathCandidateHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func (c SearchConstraints) blockedNode(nodeID string) bool {
	if len(c.BlockedNodes) == 0 {
		return false
	}
	_, ok := c.BlockedNodes[nodeID]
	return ok
}

func (c SearchConstraints) blockedEdge(edgeID string) bool {
	if len(c.BlockedEdges) == 0 {
		return false
	}
	_, ok := c.BlockedEdges[edgeID]
	return ok
}

func (g *Graph) KShortestPaths(start, goal string, ctx rctx.Context, optimizeBy string, limit int) []PathResult {
	limit = normalizeRouteLimit(limit)
	if limit <= 0 {
		return nil
	}

	nodes, edges := g.Astar(start, goal, ctx, optimizeBy)
	if len(nodes) == 0 {
		return nil
	}

	results := []PathResult{{
		Nodes: append([]string(nil), nodes...),
		Edges: cloneEdgeMap(edges),
	}}
	results[0].Score = g.scorePath(results[0].Nodes, results[0].Edges, ctx, optimizeBy)

	accepted := map[string]struct{}{
		pathKey(results[0].Nodes): {},
	}
	candidates := &pathCandidateHeap{}
	heap.Init(candidates)
	queued := make(map[string]struct{})

	for i := 0; i < limit-1; i++ {
		base := results[i]
		for spurIndex := 0; spurIndex < len(base.Nodes)-1; spurIndex++ {
			rootPath := append([]string(nil), base.Nodes[:spurIndex+1]...)
			constraints := SearchConstraints{
				BlockedNodes: make(map[string]struct{}),
				BlockedEdges: make(map[string]struct{}),
			}

			for _, nodeID := range rootPath[:len(rootPath)-1] {
				constraints.BlockedNodes[nodeID] = struct{}{}
			}

			for _, route := range results {
				if len(route.Nodes) <= spurIndex+1 {
					continue
				}
				if !samePrefix(route.Nodes, rootPath) {
					continue
				}

				nextNode := route.Nodes[spurIndex+1]
				if edge, ok := route.Edges[nextNode]; ok {
					constraints.BlockedEdges[edge.ID] = struct{}{}
				}
			}

			spurNodes, spurEdges := g.AstarWithConstraints(base.Nodes[spurIndex], goal, ctx, optimizeBy, constraints)
			if len(spurNodes) == 0 {
				continue
			}

			candidateNodes := append(rootPath, spurNodes[1:]...)
			key := pathKey(candidateNodes)
			if _, ok := accepted[key]; ok {
				continue
			}
			if _, ok := queued[key]; ok {
				continue
			}

			candidateEdges := cloneEdgeMap(base.Edges)
			for edgeKey, edge := range spurEdges {
				candidateEdges[edgeKey] = edge
			}

			candidate := pathCandidate{
				PathResult: PathResult{
					Nodes: candidateNodes,
					Edges: candidateEdges,
					Score: g.scorePath(candidateNodes, candidateEdges, ctx, optimizeBy),
				},
			}

			heap.Push(candidates, candidate)
			queued[key] = struct{}{}
		}

		if candidates.Len() == 0 {
			break
		}

		var next PathResult
		found := false
		for candidates.Len() > 0 {
			candidate := heap.Pop(candidates).(pathCandidate)
			key := pathKey(candidate.Nodes)
			if _, ok := accepted[key]; ok {
				continue
			}
			next = candidate.PathResult
			found = true
			accepted[key] = struct{}{}
			break
		}

		if !found {
			break
		}

		results = append(results, next)
	}

	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

func (g *Graph) scorePath(nodes []string, edges map[string]models.Edge, ctx rctx.Context, optimizeBy string) float64 {
	if len(nodes) < 2 {
		return 0
	}

	var score float64
	for i := 0; i < len(nodes)-1; i++ {
		edge, ok := edges[nodes[i+1]]
		if !ok {
			continue
		}
		score += EdgeWeight(edge, ctx, optimizeBy)
	}

	return score
}

func pathKey(nodes []string) string {
	return strings.Join(nodes, "->")
}

func samePrefix(nodes, prefix []string) bool {
	if len(nodes) < len(prefix) {
		return false
	}

	for i := range prefix {
		if nodes[i] != prefix[i] {
			return false
		}
	}

	return true
}

func cloneEdgeMap(src map[string]models.Edge) map[string]models.Edge {
	if len(src) == 0 {
		return map[string]models.Edge{}
	}

	dst := make(map[string]models.Edge, len(src))
	for key, edge := range src {
		dst[key] = edge
	}
	return dst
}

func normalizeRouteLimit(limit int) int {
	switch {
	case limit <= 0:
		return 5
	case limit > 5:
		return 5
	default:
		return limit
	}
}
