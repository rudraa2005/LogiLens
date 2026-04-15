package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rudraa2005/LogiLens/routing-service/models"
)

type RouteRepository struct {
	db *pgxpool.Pool
}

func NewRouteRepository(db *pgxpool.Pool) *RouteRepository {
	return &RouteRepository{
		db: db,
	}
}

func (r *RouteRepository) CreateRoute(ctx context.Context, route models.Route) (string, error) {
	query := `
		INSERT INTO routes(
			id,
			source_node_id,
			destination_node_id,
			total_distance,
			total_time,
			total_cost,
			version,
			is_active,
			parent_route_id
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id;
	`

	version := route.Version
	if version <= 0 {
		version = 1
	}
	isActive := true
	if route.Version > 0 || route.ParentRouteID != "" {
		isActive = route.IsActive
		if !route.IsActive {
			isActive = false
		}
	}

	var id string
	err := r.db.QueryRow(
		ctx,
		query,
		route.ID,
		route.SourceNodeID,
		route.DestinationNodeID,
		route.TotalDistance,
		route.TotalTime,
		route.TotalCost,
		version,
		isActive,
		nullableString(route.ParentRouteID),
	).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *RouteRepository) CreateRouteSteps(ctx context.Context, routeID string, steps []models.RouteStep) error {
	return insertRouteSteps(ctx, r.db, routeID, steps)
}

func (r *RouteRepository) CreateNewVersion(ctx context.Context, oldRouteID string, route models.Route, steps []models.RouteStep) (string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	oldRoute, err := routeByID(ctx, tx, oldRouteID)
	if err != nil {
		return "", err
	}

	if _, err := tx.Exec(ctx, `UPDATE routes SET is_active = FALSE WHERE id = $1`, oldRouteID); err != nil {
		return "", fmt.Errorf("deactivate previous route version: %w", err)
	}

	query := `
		INSERT INTO routes(
			id,
			source_node_id,
			destination_node_id,
			total_distance,
			total_time,
			total_cost,
			version,
			is_active,
			parent_route_id
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id
	`

	version := oldRoute.Version + 1
	if version <= 1 {
		version = 2
	}
	newID := route.ID
	if newID == "" || newID == oldRouteID {
		newID = uuid.New().String()
	}

	var newRouteID string
	if err := tx.QueryRow(
		ctx,
		query,
		newID,
		route.SourceNodeID,
		route.DestinationNodeID,
		route.TotalDistance,
		route.TotalTime,
		route.TotalCost,
		version,
		true,
		oldRouteID,
	).Scan(&newRouteID); err != nil {
		return "", fmt.Errorf("insert route version: %w", err)
	}

	if err := insertRouteSteps(ctx, tx, newRouteID, steps); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return newRouteID, nil
}

func (r *RouteRepository) GetAllEdges(ctx context.Context) ([]models.Edge, error) {
	query := `SELECT id, from_node_id, to_node_id, mode_id, distance, base_time, base_cost, geometry FROM edges`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []models.Edge

	for rows.Next() {
		var e models.Edge
		err := rows.Scan(
			&e.ID,
			&e.From,
			&e.To,
			&e.ModeID,
			&e.Distance,
			&e.Time,
			&e.Cost,
			&e.Geometry,
		)
		if err != nil {
			return nil, err
		}
		edges = append(edges, e)
	}

	return edges, nil
}

func (r *RouteRepository) GetRouteByID(ctx context.Context, routeID string) (models.Route, error) {
	return routeByID(ctx, r.db, routeID)
}

func (r *RouteRepository) ListRecentRoutes(ctx context.Context, limit int) ([]models.Route, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, source_node_id, destination_node_id, total_distance, total_time, total_cost,
		       COALESCE(created_at, NOW()), COALESCE(version, 1), COALESCE(is_active, TRUE),
		       COALESCE(parent_route_id::text, '')
		FROM routes
		WHERE COALESCE(is_active, TRUE) = TRUE
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	routes := make([]models.Route, 0)
	for rows.Next() {
		var route models.Route
		if err := rows.Scan(
			&route.ID,
			&route.SourceNodeID,
			&route.DestinationNodeID,
			&route.TotalDistance,
			&route.TotalTime,
			&route.TotalCost,
			&route.CreatedAt,
			&route.Version,
			&route.IsActive,
			&route.ParentRouteID,
		); err != nil {
			return nil, err
		}
		routes = append(routes, route)
	}

	return routes, nil
}

func (r *RouteRepository) CountRouteVersionsSince(ctx context.Context, routeID string, since time.Time) (int, error) {
	query := `
		WITH RECURSIVE route_family AS (
			SELECT id, parent_route_id, created_at
			FROM routes
			WHERE id = $1
			UNION
			SELECT r.id, r.parent_route_id, r.created_at
			FROM routes r
			JOIN route_family rf
				ON r.id = rf.parent_route_id OR r.parent_route_id = rf.id
		)
		SELECT COUNT(DISTINCT id)
		FROM route_family
		WHERE COALESCE(created_at, NOW()) >= $2
	`

	var count int
	if err := r.db.QueryRow(ctx, query, routeID, since).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func routeByID(ctx context.Context, querier queryRower, routeID string) (models.Route, error) {
	query := `
		SELECT id, source_node_id, destination_node_id, total_distance, total_time, total_cost,
		       COALESCE(created_at, NOW()), COALESCE(version, 1), COALESCE(is_active, TRUE),
		       COALESCE(parent_route_id::text, '')
		FROM routes
		WHERE id = $1
	`

	var route models.Route
	err := querier.QueryRow(ctx, query, routeID).Scan(
		&route.ID,
		&route.SourceNodeID,
		&route.DestinationNodeID,
		&route.TotalDistance,
		&route.TotalTime,
		&route.TotalCost,
		&route.CreatedAt,
		&route.Version,
		&route.IsActive,
		&route.ParentRouteID,
	)
	if err != nil {
		return models.Route{}, err
	}
	return route, nil
}

func insertRouteSteps(ctx context.Context, tx batchSender, routeID string, steps []models.RouteStep) error {
	batch := &pgx.Batch{}

	query := `
		INSERT INTO route_steps(
			id,
			route_id,
			from_node_id,
			to_node_id,
			edge_id,
			sequence,
			distance,
			time,
			cost,
			mode_id
		)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`

	for _, step := range steps {
		batch.Queue(
			query,
			step.ID,
			routeID,
			step.FromNodeID,
			step.ToNodeID,
			step.EdgeID,
			step.Sequence,
			step.Distance,
			step.Time,
			step.Cost,
			step.ModeID,
		)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(steps); i++ {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}

	return nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type batchSender interface {
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}
