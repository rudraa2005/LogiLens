package server

import (
	"context"
	"log"
	"strings"
	"time"

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
	departure := time.Unix(req.GetDepartureTimeUnix(), 0).UTC()
	if req.GetDepartureTimeUnix() == 0 {
		departure = time.Now().UTC()
		log.Printf("route request missing departure_time_unix; defaulting to current UTC time %s", departure.Format(time.RFC3339))
	}

	routeResp, err := s.service.ComputeRoute(ctx, services.RouteRequest{
		Source:      req.GetSource(),
		Destination: req.GetDestination(),
		OptimizeBy:  strings.ToLower(req.GetOptimizeBy().String()),
	}, departure)
	if err != nil {
		return nil, err
	}

	return routeResp.ToProto(), nil
}
