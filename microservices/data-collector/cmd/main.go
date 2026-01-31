package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/config"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/grpc_client"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/huma_api"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/kafka_producer"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/observability"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/opensky"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/repository"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/worker"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/api"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

func main() {
	log.Println("Starting Data Collector Service...")

	httpPort := config.GetServerPort()
	neo4jURI := config.GetNeo4jURI()
	neoUser, neoPass := config.GetNeo4jAuth()
	userManagerHost := config.GetUserManagerHost()
	osUser, osPass := config.GetOpenSkyAuth()
	kafkaBroker := config.GetKafkaBroker()
	kafkaTopic := config.GetKafkaTopic()
	collectionInterval := config.GetCollectionInterval()
	serviceName := config.GetServiceName()
	nodeName := config.GetNodeName()

	monitor := observability.NewMonitor(serviceName, nodeName)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Connecting to Kafka at %s...", kafkaBroker)
	producerAdapter, err := kafka_producer.NewKafkaProducer(kafkaBroker, kafkaTopic)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to create Kafka producer: %v", err)
	}
	log.Println("Kafka Producer connected")

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

	app := api.NewApplication(repo, userClient, openSkyClient, producerAdapter, collectionInterval)

	backgroundWorker := worker.NewTickerWorker(app, collectionInterval, monitor)
	go backgroundWorker.Start(ctx)

	apiHandler := huma_api.NewAPIHandler(app)
	mux := http.NewServeMux()

	mux.Handle("/metrics", promhttp.Handler())

	humaConfig := huma.DefaultConfig("Data Collector API", "1.0.0")
	humaConfig.Info.Description = `Microservice for collecting and analyzing flight data.`
	humaConfig.DocsPath = "/docs/data"
	humaConfig.OpenAPIPath = "/schemas/data"

	humaAPI := humago.New(mux, humaConfig)
	apiHandler.RegisterRoutes(humaAPI)

	metricsMiddleware := middleware.NewMetricsMiddleware(monitor)
	finalHandler := metricsMiddleware(mux)

	serverAddr := fmt.Sprintf(":%s", httpPort)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      finalHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("HTTP Server listening on port %s", httpPort)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("Received signal: %v. Starting graceful shutdown...", sig)

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP Server shutdown error: %v", err)
	} else {
		log.Println("HTTP Server stopped gracefully.")
	}

	log.Println("Closing resources...")

	if err := repo.Close(context.Background()); err != nil {
		log.Printf("Error closing Neo4j: %v", err)
	} else {
		log.Println("Neo4j connection closed.")
	}

	producerAdapter.Close()
	log.Println("Kafka Producer closed.")

	log.Println("Data Collector Shutdown complete.")
}
