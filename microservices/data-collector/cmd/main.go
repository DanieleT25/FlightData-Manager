package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/config"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/grpc_client"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/huma_api"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/opensky"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/repository"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/worker"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/api"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

func main() {
	httpPort := config.GetServerPort()
	neo4jURI := config.GetNeo4jURI()
	neoUser, neoPass := config.GetNeo4jAuth()
	userManagerHost := config.GetUserManagerHost()
	osUser, osPass := config.GetOpenSkyAuth()
	collectionInterval := config.GetCollectionInterval()

	ctx := context.Background()
	log.Println("Starting Data Collector Service")

	log.Printf("Connecting to Neo4j at %s...", neo4jURI)
	repo, err := repository.NewNeo4jRepository(ctx, neo4jURI, neoUser, neoPass)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to connect to Neo4j: %v", err)
	}
	log.Println("Neo4j connected")

	log.Printf("Connecting to User Manager at %s...", userManagerHost)
	userClient, err := grpc_client.NewUserClientAdapter(userManagerHost)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to create gRPC client: %v", err)
	}

	openSkyClient := opensky.NewOpenSkyClient(osUser, osPass)
	app := api.NewApplication(repo, userClient, openSkyClient, collectionInterval)
	backgroundWorker := worker.NewTickerWorker(app, collectionInterval)

	go backgroundWorker.Start(ctx)

	apiHandler := huma_api.NewAPIHandler(app)
	mux := http.NewServeMux()

	humaConfig := huma.DefaultConfig("Data Collector API", "1.0.0")
	humaConfig.Info.Description = `Microservice for collecting and analyzing flight data. It manages user interests and periodically downloads data from OpenSky Network.`

	humaConfig.DocsPath = "/docs/data"
	humaConfig.OpenAPIPath = "/schemas/data"

	humaAPI := humago.New(mux, humaConfig)

	apiHandler.RegisterRoutes(humaAPI)

	serverAddr := fmt.Sprintf(":%s", httpPort)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("HTTP Server listening on port %s", httpPort)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
