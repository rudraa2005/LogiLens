package worker

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rudraa2005/LogiLens/routing-service/models"
	"github.com/rudraa2005/LogiLens/routing-service/services"
)

const (
	defaultReoptimizationInterval = 3 * time.Hour
	defaultImprovementThreshold   = 0.07
	defaultReoptimizationBatch    = 50
	defaultReoptimizationCooldown = 2 * time.Hour
	defaultMaxDailyUpdates        = 3
)

type RouteRepository interface {
	ListRecentRoutes(ctx context.Context, limit int) ([]models.Route, error)
	GetRouteByID(ctx context.Context, routeID string) (models.Route, error)
	CreateNewVersion(ctx context.Context, oldRouteID string, route models.Route, steps []models.RouteStep) (string, error)
	CountRouteVersionsSince(ctx context.Context, routeID string, since time.Time) (int, error)
}

type RoutePlanner interface {
	RecomputeStoredRoute(ctx context.Context, stored models.Route, optimizeBy string) (services.RouteResponse, error)
}

type Notifier interface {
	NotifyRouteUpdated(ctx context.Context, routeID string, route services.RouteResponse) error
}

type ReoptimizationWorker struct {
	repo                 RouteRepository
	planner              RoutePlanner
	notifier             Notifier
	interval             time.Duration
	improvementThreshold float64
	batchSize            int
	cooldown             time.Duration
	maxDailyUpdates      int
	logger               *log.Logger
}

type RouteComparison struct {
	Improved         bool
	TimeImprovement  float64
	CostImprovement  float64
	DistanceDelta    float64
	ImprovementRatio float64
}

func NewReoptimizationWorker(repo RouteRepository, planner RoutePlanner, notifier Notifier) *ReoptimizationWorker {
	return &ReoptimizationWorker{
		repo:                 repo,
		planner:              planner,
		notifier:             notifier,
		interval:             reoptimizationIntervalFromEnv(),
		improvementThreshold: reoptimizationThresholdFromEnv(),
		batchSize:            reoptimizationBatchSizeFromEnv(),
		cooldown:             reoptimizationCooldownFromEnv(),
		maxDailyUpdates:      reoptimizationMaxDailyUpdatesFromEnv(),
		logger:               log.New(os.Stdout, "reoptimizer: ", log.LstdFlags),
	}
}

func (w *ReoptimizationWorker) Start(ctx context.Context) {
	if w == nil || w.repo == nil || w.planner == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		w.runOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.runOnce(ctx)
			}
		}
	}()
}

func (w *ReoptimizationWorker) runOnce(ctx context.Context) {
	routes, err := w.repo.ListRecentRoutes(ctx, w.batchSize)
	if err != nil {
		w.logger.Printf("list routes failed: %v", err)
		return
	}

	for _, route := range routes {
		if err := w.RecomputeRoute(ctx, route.ID); err != nil {
			w.logger.Printf("recompute route %s failed: %v", route.ID, err)
		}
	}
}

func (w *ReoptimizationWorker) RecomputeRoute(ctx context.Context, routeID string) error {
	stored, err := w.repo.GetRouteByID(ctx, routeID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if w.cooldown > 0 && !stored.CreatedAt.IsZero() && now.Sub(stored.CreatedAt) < w.cooldown {
		return nil
	}
	if w.maxDailyUpdates > 0 {
		count, err := w.repo.CountRouteVersionsSince(ctx, stored.ID, now.Add(-24*time.Hour))
		if err != nil {
			return err
		}
		if count >= w.maxDailyUpdates {
			return nil
		}
	}

	recomputed, err := w.planner.RecomputeStoredRoute(ctx, stored, "time")
	if err != nil {
		return err
	}

	comparison := CompareRoutes(stored, recomputed)
	if !comparison.Improved || comparison.ImprovementRatio < w.improvementThreshold {
		return nil
	}

	replacement := models.Route{
		ID:                recomputed.RouteID,
		SourceNodeID:      recomputed.SourceNodeID,
		DestinationNodeID: recomputed.DestinationNodeID,
		TotalDistance:     recomputed.TotalDistance,
		TotalTime:         recomputed.TotalTime,
		TotalCost:         recomputed.TotalCost,
		CreatedAt:         stored.CreatedAt,
	}
	if replacement.ID == "" {
		replacement.ID = stored.ID
	}

	steps := make([]models.RouteStep, 0, len(recomputed.Steps))
	for i, step := range recomputed.Steps {
		step.Sequence = i
		steps = append(steps, step)
	}

	newRouteID, err := w.repo.CreateNewVersion(ctx, stored.ID, replacement, steps)
	if err != nil {
		return err
	}

	if w.notifier != nil {
		recomputed.RouteID = newRouteID
		_ = w.notifier.NotifyRouteUpdated(ctx, newRouteID, recomputed)
	}

	return nil
}

func CompareRoutes(old models.Route, updated services.RouteResponse) RouteComparison {
	timeImprovement := old.TotalTime - updated.TotalTime
	costImprovement := old.TotalCost - updated.TotalCost
	improvementRatio := 0.0
	if old.TotalTime > 0 {
		improvementRatio = timeImprovement / old.TotalTime
	}

	return RouteComparison{
		Improved:         timeImprovement > 0 || costImprovement > 0,
		TimeImprovement:  timeImprovement,
		CostImprovement:  costImprovement,
		DistanceDelta:    updated.TotalDistance - old.TotalDistance,
		ImprovementRatio: improvementRatio,
	}
}

type LogNotifier struct {
	logger *log.Logger
}

func NewLogNotifier() *LogNotifier {
	return &LogNotifier{logger: log.New(os.Stdout, "route-updates: ", log.LstdFlags)}
}

func (n *LogNotifier) NotifyRouteUpdated(ctx context.Context, routeID string, route services.RouteResponse) error {
	if n == nil || n.logger == nil {
		return nil
	}
	n.logger.Printf(
		"route %s updated: time=%.2f cost=%.2f distance=%.2f",
		routeID,
		route.TotalTime,
		route.TotalCost,
		route.TotalDistance,
	)
	return nil
}

func reoptimizationIntervalFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("REOPTIMIZER_INTERVAL_HOURS"))
	if raw == "" {
		return defaultReoptimizationInterval
	}
	hours, err := strconv.Atoi(raw)
	if err != nil || hours <= 0 {
		return defaultReoptimizationInterval
	}
	return time.Duration(hours) * time.Hour
}

func reoptimizationThresholdFromEnv() float64 {
	raw := strings.TrimSpace(os.Getenv("REOPTIMIZER_THRESHOLD_RATIO"))
	if raw == "" {
		return defaultImprovementThreshold
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 {
		return defaultImprovementThreshold
	}
	return value
}

func reoptimizationBatchSizeFromEnv() int {
	raw := strings.TrimSpace(os.Getenv("REOPTIMIZER_BATCH_SIZE"))
	if raw == "" {
		return defaultReoptimizationBatch
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultReoptimizationBatch
	}
	return value
}

func reoptimizationCooldownFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv("REOPTIMIZER_COOLDOWN_MINUTES"))
	if raw == "" {
		return defaultReoptimizationCooldown
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultReoptimizationCooldown
	}
	return time.Duration(value) * time.Minute
}

func reoptimizationMaxDailyUpdatesFromEnv() int {
	raw := strings.TrimSpace(os.Getenv("REOPTIMIZER_MAX_DAILY_UPDATES"))
	if raw == "" {
		return defaultMaxDailyUpdates
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return defaultMaxDailyUpdates
	}
	return value
}
