package repository

import (
	"context"

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
		INSERT INTO routes(id, source_node_id, destination_node_id, total_distance,total_time,total_cost)
		VALUES($1,$2,$3,$4,$5,$6)
		RETURNING id;
	`

	var id string
	err := r.db.QueryRow(ctx, query, route.ID, route.SourceNodeID, route.DestinationNodeID, route.TotalDistance, route.TotalTime, route.TotalCost).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *RouteRepository) CreateRouteSteps(ctx context.Context, routeID string, steps []models.RouteStep) error {

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

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	// Execute all queued queries
	for i := 0; i < len(steps); i++ {
		_, err := br.Exec()
		if err != nil {
			return err
		}
	}

	return nil
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
