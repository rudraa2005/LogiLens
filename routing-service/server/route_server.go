package server

import (
	"context"
	"strings"

	pb "github.com/rudraa2005/LogiLens/proto"
	"github.com/rudraa2005/LogiLens/routing-service/services"
)

type RouteServer struct {
	pb.UnimplementedRouteServiceServer
	service *services.RouteService
}

func NewRouteServer(service *services.RouteService) *RouteServer {
	return &RouteServer{service: service}
}

func (s *RouteServer) ComputeRoute(ctx context.Context, req *pb.RouteRequest) (*pb.RouteResponse, error) {
	routeResp, err := s.service.ComputeRoute(ctx, services.RouteRequest{
		Source:      req.GetSource(),
		Destination: req.GetDestination(),
		OptimizeBy:  strings.ToLower(req.GetOptimizeBy().String()),
	})
	if err != nil {
		return nil, err
	}

	return routeResp.ToProto(), nil
}
