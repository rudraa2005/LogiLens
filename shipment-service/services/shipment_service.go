package services

import (
	"context"

	"github.com/rudraa2005/LogiLens/shipment-service/models"
	"github.com/rudraa2005/LogiLens/shipment-service/repository"
)

type ShipmentService struct {
	repo *repository.ShipmentRepo
}

func NewShipmentService(repo *repository.ShipmentRepo) *ShipmentService {
	return &ShipmentService{
		repo: repo,
	}
}

func (svc *ShipmentService) CreateShipment(ctx context.Context, userID string, origin string, destination string) (string, *models.Shipment, error) {

	shipment := &models.Shipment{
		UserID:      userID,
		Origin:      origin,
		Destination: destination,
		Status:      "CREATED",
	}

	id, err := svc.repo.CreateShipment(ctx, shipment)
	if err != nil {
		return "", nil, err
	}

	return id, shipment, nil
}

func (svc *ShipmentService) GetShipment(ctx context.Context, shipmentID string) (*models.Shipment, error) {

	shipment, err := svc.repo.GetShipment(ctx, shipmentID)
	if err != nil {
		return nil, err
	}

	return shipment, err
}

func (svc *ShipmentService) MarkInTransit(ctx context.Context, shipment_id string) (*models.Shipment, error) {
	shipment, err := svc.repo.MarkInTransit(ctx, shipment_id)
	if err != nil {
		return nil, err
	}
	return shipment, nil
}

func (svc *ShipmentService) MarkDelivered(ctx context.Context, shipment_id string) (*models.Shipment, error) {
	shipment, err := svc.repo.MarkDelivered(ctx, shipment_id)
	if err != nil {
		return nil, err
	}
	return shipment, nil
}
