package main

import (
	"context"
	"log"
	"net"

	proto "github.com/rudraa2005/LogiLens/proto"
	"github.com/rudraa2005/LogiLens/routing-service/db"
	"github.com/rudraa2005/LogiLens/routing-service/graph"
	"github.com/rudraa2005/LogiLens/routing-service/repository"
	"github.com/rudraa2005/LogiLens/routing-service/server"
	"github.com/rudraa2005/LogiLens/routing-service/services"
	"google.golang.org/grpc"
)

func main() {
	listenAddr := ":50052"

	pool, err := db.NewPool()
	if err != nil {
		log.Fatal("DB connection failed:", err)
	}
	defer pool.Close()

	routeRepo := repository.NewRouteRepository(pool)
	edges, err := routeRepo.GetAllEdges(context.Background())
	if err != nil {
		log.Fatal("failed to load edges:", err)
	}

	g := graph.BuildGraph(nil, edges)
	log.Printf("loaded %d edges and inferred %d graph nodes", len(edges), len(g.Nodes))
	if len(g.Nodes) == 0 {
		log.Fatal("routing graph is empty; seed the edges table with route geometry before running")
	}
	routeService := services.NewRouteService(routeRepo, g)

	grpcServer := grpc.NewServer()
	proto.RegisterRouteServiceServer(grpcServer, server.NewRouteServer(routeService))

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal("listen error:", err)
	}

	log.Println("routing-service listening on", listenAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal("serve error:", err)
	}
}
