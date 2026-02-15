package services

import (
	"context"
	"time"

	"github.com/rudraa2005/LogiLens/api-gateway-service/shipmentpb"
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
