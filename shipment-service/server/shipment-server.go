package server

import (
	"context"

	pb "github.com/rudraa2005/LogiLens/proto"
	"github.com/rudraa2005/LogiLens/shipment-service/services"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//gRPC implementation

type ShipmentServer struct {
	pb.UnimplementedShipmentServiceServer
	shipmentService *services.ShipmentService
}

func NewShipmentServer(s *services.ShipmentService) *ShipmentServer {
	return &ShipmentServer{
		shipmentService: s,
	}
}

func (s *ShipmentServer) CreateShipment(ctx context.Context, req *pb.CreateShipmentRequest) (*pb.ShipmentResponse, error) {
	shipmentID, shipment, err := s.shipmentService.CreateShipment(ctx, req.UserId, req.Origin, req.Destination)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create shipment")
	}

	return &pb.ShipmentResponse{ShipmentId: shipmentID, Status: shipment.Status}, nil
}

func (s *ShipmentServer) GetShipment(ctx context.Context, req *pb.GetShipmentRequest) (*pb.ShipmentResponse, error) {

	shipment, err := s.shipmentService.GetShipment(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	return &pb.ShipmentResponse{
		ShipmentId:  shipment.ShipmentID,
		Origin:      shipment.Origin,
		Destination: shipment.Destination,
		Status:      shipment.Status,
	}, nil
}

func (s *ShipmentServer) MarkInTransit(ctx context.Context, req *pb.GetShipmentRequest) (*pb.ShipmentResponse, error) {
	shipment, err := s.shipmentService.MarkInTransit(ctx, req.ShipmentId)
	if err != nil {
		return nil, err
	}
	return &pb.ShipmentResponse{
		ShipmentId:  shipment.ShipmentID,
		UserId:      shipment.UserID,
		Origin:      shipment.Origin,
		Destination: shipment.Destination,
		Status:      shipment.Status,
	}, nil
}
