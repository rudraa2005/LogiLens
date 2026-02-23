package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rudraa2005/LogiLens/shipment-service/models"
)

type ShipmentRepo struct {
	db *pgxpool.Pool
}

func NewShipmentRepository(db *pgxpool.Pool) *ShipmentRepo {
	return &ShipmentRepo{
		db: db,
	}
}

func (s *ShipmentRepo) CreateShipment(ctx context.Context, shipment *models.Shipment) (string, error) {

	query := `
		INSERT INTO shipments(user_id,origin,destination,status) 
		VALUES ($1,$2,$3,$4)
		RETURNING shipment_id
	`

	err := s.db.QueryRow(context.Background(), query, shipment.UserID, shipment.Origin, shipment.Destination, shipment.Status).Scan(&shipment.ShipmentID)
	if err != nil {
		return "", err
	}
	return shipment.ShipmentID, nil
}

func (s *ShipmentRepo) GetShipment(ctx context.Context, shipmentID string) (*models.Shipment, error) {
	query := `
		SELECT shipment_id,origin,destination,status
		FROM shipments
		WHERE user_id = $1
	`

	var shipment models.Shipment
	err := s.db.QueryRow(ctx, query, shipmentID).Scan(&shipment.ShipmentID, &shipment.Origin, &shipment.Destination, &shipment.Status)
	if err != nil {
		return nil, err
	}
	return &shipment, nil
}

func (s *ShipmentRepo) MarkInTransit(ctx context.Context, shipment_id string) (*models.Shipment, error) {
	query := `
		UPDATE shipments
		SET status = 'in_transit',
			updated_at= now()
		WHERE shipment_id = $1
			AND status = 'CREATED'
		RETURNING shipment_id, user_id, origin,destination,status
	`
	var shipment models.Shipment
	err := s.db.QueryRow(ctx, query, shipment_id).Scan(shipment.ShipmentID, shipment.UserID, shipment.Origin, shipment.Destination, shipment.Status)
	if err != nil {
		return nil, err
	}
	return &shipment, nil
}

func (s *ShipmentRepo) MarkDelivered(ctx context.Context, shipment_id string) (*models.Shipment, error) {

	query := `
		UPDATE shipments
		SET status = 'delivered',
			updated_at = now()
		WHERE shipment_id = $1
			AND status = 'in_transit'
		RETURNING shipment_id, user_id, origin,destination,status
	`
	var shipment models.Shipment
	err := s.db.QueryRow(ctx, query, shipment_id).Scan(shipment.ShipmentID, shipment.UserID, shipment.Origin, shipment.Destination, shipment.Status)
	if err != nil {
		return nil, err
	}
	return &shipment, nil
}
