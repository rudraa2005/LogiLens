package worker

import (
	"context"
	"testing"
	"time"

	"github.com/rudraa2005/LogiLens/routing-service/models"
	"github.com/rudraa2005/LogiLens/routing-service/services"
)

type repoStub struct {
	route    models.Route
	created  models.Route
	steps    []models.RouteStep
	oldRoute string
}

func (r *repoStub) ListRecentRoutes(ctx context.Context, limit int) ([]models.Route, error) {
	return []models.Route{r.route}, nil
}

func (r *repoStub) GetRouteByID(ctx context.Context, routeID string) (models.Route, error) {
	return r.route, nil
}

func (r *repoStub) CreateNewVersion(ctx context.Context, oldRouteID string, route models.Route, steps []models.RouteStep) (string, error) {
	r.oldRoute = oldRouteID
	r.created = route
	r.steps = steps
	return "route-2", nil
}

type plannerStub struct {
	resp services.RouteResponse
}

func (p *plannerStub) RecomputeStoredRoute(ctx context.Context, stored models.Route, optimizeBy string) (services.RouteResponse, error) {
	return p.resp, nil
}

func TestCompareRoutesDetectsImprovement(t *testing.T) {
	oldRoute := models.Route{TotalTime: 20, TotalCost: 10, TotalDistance: 30}
	newRoute := services.RouteResponse{TotalTime: 15, TotalCost: 8, TotalDistance: 31}

	comparison := CompareRoutes(oldRoute, newRoute)
	if !comparison.Improved {
		t.Fatal("expected improved route")
	}
	if comparison.ImprovementRatio <= 0 {
		t.Fatalf("expected positive improvement ratio, got %v", comparison.ImprovementRatio)
	}
}

func TestRecomputeRouteCreatesNewVersionWhenImproved(t *testing.T) {
	repo := &repoStub{
		route: models.Route{
			ID:                "route-1",
			SourceNodeID:      "a",
			DestinationNodeID: "b",
			TotalTime:         20,
			TotalCost:         10,
			TotalDistance:     15,
			CreatedAt:         time.Now().UTC(),
		},
	}
	planner := &plannerStub{
		resp: services.RouteResponse{
			RouteID:           "route-1",
			SourceNodeID:      "a",
			DestinationNodeID: "b",
			TotalTime:         15,
			TotalCost:         9,
			TotalDistance:     15,
			Steps: []models.RouteStep{
				{ID: "step-1", EdgeID: "edge-1"},
			},
		},
	}

	worker := NewReoptimizationWorker(repo, planner, nil)
	worker.improvementThreshold = 0.01

	if err := worker.RecomputeRoute(context.Background(), "route-1"); err != nil {
		t.Fatalf("recompute route: %v", err)
	}
	if repo.oldRoute != "route-1" {
		t.Fatalf("expected previous route id route-1, got %q", repo.oldRoute)
	}
	if len(repo.steps) != 1 {
		t.Fatalf("expected steps to be replaced, got %d", len(repo.steps))
	}
}
