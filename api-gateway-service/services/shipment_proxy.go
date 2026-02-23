package services

import (
	"context"
	"time"

	shipmentpb "github.com/rudraa2005/LogiLens/proto"
)

type ShipmentService struct {
	client shipmentpb.ShipmentServiceClient
}

func NewShipmentService(client shipmentpb.ShipmentServiceClient) *ShipmentService {
	return &ShipmentService{
		client: client,
	}
}

func (s *ShipmentService) CreateShipment(ctx context.Context, source string, destination string, userId string) (*shipmentpb.ShipmentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req := &shipmentpb.CreateShipmentRequest{
		UserId:      userId,
		Origin:      source,
		Destination: destination,
	}

	return s.client.CreateShipment(ctx, req)
}

func (s *ShipmentService) GetShipment(ctx context.Context, userID string) (*shipmentpb.ShipmentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req := &shipmentpb.GetShipmentRequest{
		UserId: userID,
	}
	return s.client.GetShipment(ctx, req)
}

func (s *ShipmentService) MarkInTransit(ctx context.Context, shipmentID string) (*shipmentpb.ShipmentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req := &shipmentpb.GetShipmentRequest{
		ShipmentId: shipmentID,
	}
	return s.client.MarkInTransit(ctx, req)
}

func (s *ShipmentService) MarkDelivered(ctx context.Context, shipmentID string) (*shipmentpb.ShipmentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req := &shipmentpb.GetShipmentRequest{
		ShipmentId: shipmentID,
	}
	return s.client.MarkInTransit(ctx, req)
}
