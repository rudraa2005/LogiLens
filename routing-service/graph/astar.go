package graph

import (
	"container/heap"
	"math"

	"github.com/rudraa2005/LogiLens/routing-service/context"
	"github.com/rudraa2005/LogiLens/routing-service/models"
)

type Item struct {
	Node     string
	Priority float64
	Index    int
}

type PriorityQueue []*Item

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].Priority < pq[j].Priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*Item)
	item.Index = len(*pq)
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]

	return item
}

func (g *Graph) heuristic(a, b string) float64 {
	n1 := g.Nodes[a]
	n2 := g.Nodes[b]

	dx := n1.Latitude - n2.Latitude
	dy := n1.Longitude - n2.Longitude

	return math.Sqrt(dx*dx + dy*dy)
}

func (g *Graph) Astar(start string, goal string, ctx context.Context, optimizeBy string) ([]string, map[string]models.Edge) {
	openSet := &PriorityQueue{}
	heap.Init(openSet)

	heap.Push(openSet, &Item{
		Node:     start,
		Priority: 0,
	})

	gScore := make(map[string]float64)
	cameFrom := make(map[string]string)
	cameFromEdge := make(map[string]models.Edge)

	for node := range g.Nodes {
		gScore[node] = math.Inf(1)
	}
	gScore[start] = 0
	for openSet.Len() > 0 {
		current := heap.Pop(openSet).(*Item).Node

		if current == goal {
			return g.ReconstructPath(cameFrom, cameFromEdge, current)
		}

		for _, edge := range g.Adjacency[current] {
			neighbour := edge.To
			weight := context.GetEdgeWeight(edge, ctx, optimizeBy)
			tentative := gScore[current] + weight

			if tentative < gScore[neighbour] {
				cameFrom[neighbour] = current
				cameFromEdge[neighbour] = edge

				gScore[neighbour] = tentative
				fScore := tentative + g.heuristic(neighbour, goal)

				heap.Push(openSet, &Item{
					Node:     neighbour,
					Priority: fScore,
				})
			}

		}
	}
	return nil, nil
}

func (g *Graph) ReconstructPath(cameFrom map[string]string, cameFromEdge map[string]models.Edge, current string) ([]string, map[string]models.Edge) {
	path := []string{current}

	for {
		prev, exists := cameFrom[current]

		if !exists {
			break
		}
		path = append([]string{prev}, path...)
		current = prev
	}
	return path, cameFromEdge
}
