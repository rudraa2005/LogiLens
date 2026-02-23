package main

import (
	"log"
	"net"

	proto "github.com/rudraa2005/LogiLens/proto"
	"github.com/rudraa2005/LogiLens/shipment-service/db"
	"github.com/rudraa2005/LogiLens/shipment-service/repository"
	shipmentServer "github.com/rudraa2005/LogiLens/shipment-service/server"
	"github.com/rudraa2005/LogiLens/shipment-service/services"
	"google.golang.org/grpc"
)

func main() {
	pool, err := db.NewPool()
	if err != nil {
		log.Fatal("DB connection failed:", err)
	}
	defer pool.Close()

	grpcShipmentServer := grpc.NewServer()
	shipmentRepo := repository.NewShipmentRepository(pool)
	shipmentService := services.NewShipmentService(shipmentRepo)
	proto.RegisterShipmentServiceServer(grpcShipmentServer, shipmentServer.NewShipmentServer(shipmentService))
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal("Listen Error:", err)
	}
	if err := grpcShipmentServer.Serve(lis); err != nil {
		log.Fatal("Serve error:", err)
	}
}
