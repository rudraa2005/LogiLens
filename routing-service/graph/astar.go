package graph

import (
	"container/heap"
	"math"
	"strings"
	"time"

	rctx "github.com/rudraa2005/LogiLens/routing-service/context"
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

func (g *Graph) heuristic(a, b, optimizeBy string) float64 {
	n1 := g.Nodes[a]
	n2 := g.Nodes[b]

	distanceKm := haversineDistanceKm(n1.Latitude, n1.Longitude, n2.Latitude, n2.Longitude)

	switch strings.ToLower(strings.TrimSpace(optimizeBy)) {
	case "time":
		if g.heuristicSpeedKPH <= 0 {
			return 0
		}
		return distanceKm / g.heuristicSpeedKPH * 60.0
	case "distance":
		return distanceKm
	default:
		return 0
	}
}

func (g *Graph) Astar(start string, goal string, ctx rctx.Context, optimizeBy string, departure time.Time) ([]string, map[string]models.Edge) {
	return g.AstarWithConstraints(start, goal, ctx, optimizeBy, departure, SearchConstraints{})
}

func (g *Graph) AstarWithConstraints(start string, goal string, ctx rctx.Context, optimizeBy string, departure time.Time, constraints SearchConstraints) ([]string, map[string]models.Edge) {
	openSet := &PriorityQueue{}
	heap.Init(openSet)

	heap.Push(openSet, &Item{
		Node:     start,
		Priority: 0,
	})

	gScoreCost := make(map[string]float64)
	gScoreTime := make(map[string]float64)
	cameFrom := make(map[string]string)
	cameFromEdge := make(map[string]models.Edge)

	for node := range g.Nodes {
		gScoreCost[node] = math.Inf(1)
		gScoreTime[node] = math.Inf(1)
	}
	gScoreCost[start] = 0
	gScoreTime[start] = 0
	for openSet.Len() > 0 {
		current := heap.Pop(openSet).(*Item).Node

		if current == goal {
			return g.ReconstructPath(cameFrom, cameFromEdge, current)
		}

		for _, edge := range g.Adjacency[current] {
			neighbour := edge.To
			if constraints.blockedNode(neighbour) || constraints.blockedEdge(edge.ID) {
				continue
			}

			eta := departure
			if !departure.IsZero() && !math.IsInf(gScoreTime[current], 1) {
				eta = departure.Add(time.Duration(gScoreTime[current] * float64(time.Minute)))
			}
			combinedFactor := CombinedFactor(edge, ctx, eta)
			travelTime := edge.Time * combinedFactor
			weight := edge.Distance
			switch strings.ToLower(optimizeBy) {
			case "cost":
				weight = edge.Cost * combinedFactor
			case "distance":
				weight = edge.Distance
			default:
				weight = travelTime
			}
			tentativeCost := gScoreCost[current] + weight
			tentativeTime := gScoreTime[current] + travelTime

			if tentativeCost < gScoreCost[neighbour] {
				cameFrom[neighbour] = current
				cameFromEdge[neighbour] = edge

				gScoreCost[neighbour] = tentativeCost
				gScoreTime[neighbour] = tentativeTime
				fScore := tentativeCost + g.heuristic(neighbour, goal, optimizeBy)

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
