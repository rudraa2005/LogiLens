package main

import (
	"log"
	"net/http"

	"github.com/rudraa2005/LogiLens/api-gateway-service/db"
	"github.com/rudraa2005/LogiLens/api-gateway-service/handlers"
	"github.com/rudraa2005/LogiLens/api-gateway-service/repository"
	"github.com/rudraa2005/LogiLens/api-gateway-service/router"
	"github.com/rudraa2005/LogiLens/api-gateway-service/services"
	"github.com/rudraa2005/LogiLens/api-gateway-service/shipmentpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {

	pool, err := db.NewPool()
	if err != nil {
		log.Fatal("DB connection failed:", err)
	}
	defer pool.Close()

	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to create gRPC client: %v", err)
	}
	client := shipmentpb.NewShipmentServiceClient(conn)

	shipmentService := services.NewShipmentService(client)
	shipmentHandler := handlers.NewShipmentHandler(*shipmentService)

	userRepo := repository.NewUserRepository(pool)
	authService := services.NewAuthService(userRepo)
	authHandler := handlers.NewAuthHandler(authService)

	r := router.NewRouter(authHandler, shipmentHandler)

	log.Println("Routing....")
	http.ListenAndServe(":8080", r)
}
