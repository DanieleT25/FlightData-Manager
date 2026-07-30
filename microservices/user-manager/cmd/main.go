package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/config"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/adapters/grpc_server"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/adapters/huma_api"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/adapters/observability"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/adapters/repository"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/api"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/middleware"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	redisAddr := config.GetRedisAddr()
	redisPass := config.GetRedisPassword()
	redisDB := config.GetRedisDB()
	postgresHost := config.GetPostgresHost()
	postgresPort := config.GetPostgresPort()
	postgresUser := config.GetPostgresUser()
	postgresPassword := config.GetPostgresPassword()
	postgresDB := config.GetPostgresDB()
	postgresSSLMode := config.GetPostgresSSLMode()
	httpPort := config.GetServerPort()
	grpcPort := config.GetGRPCPort()
	serviceName := config.GetServiceName()
	nodeName := config.GetNodeName()

	ctx := context.Background()

	log.Printf("Starting User Manager Service")

	monitor := observability.NewMonitor(serviceName, nodeName)

	log.Printf("Connecting to Postgres at %s:%s (DB: %s)...", postgresHost, postgresPort, postgresDB)

	// Built through net/url rather than by string concatenation: a generated
	// password may legitimately contain ':', '?', '#', '[' or '%', all of which
	// carry syntactic meaning inside a URL and would otherwise be misread — a
	// '[' is taken as the start of an IPv6 literal, and the whole DSN fails to
	// parse. url.UserPassword percent-encodes the credentials for us.
	postgresDSN := (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(postgresUser, postgresPassword),
		Host:     net.JoinHostPort(postgresHost, postgresPort),
		Path:     postgresDB,
		RawQuery: url.Values{"sslmode": {postgresSSLMode}}.Encode(),
	}).String()

	userRepo, err := repository.NewPostgresRepository(ctx, postgresDSN)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to connect to Postgres: %v", err)
	}
	log.Println("Connected to Postgres successfully.")

	log.Printf("Connecting to Redis at %s (DB: %d)...", redisAddr, redisDB)

	idempotencyRepo, err := repository.NewRedisRepository(ctx, redisAddr, redisPass, redisDB)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis successfully.")

	userApplication := api.NewApplication(userRepo, idempotencyRepo)

	httpAdapter := huma_api.NewAPIHandler(userApplication)
	grpcAdapter := grpc_server.NewGrpcAdapter(userApplication, monitor)

	go func() {
		if err := grpcAdapter.Start(grpcPort); err != nil {
			log.Fatalf("CRITICAL: gRPC Server failed to start: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	humaConfig := huma.DefaultConfig("User Manager API", "1.0.0")
	humaConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{}
	humaConfig.Info.Description = `This **User Manager Microservice** is a core component of the distributed flight monitoring platform, responsible for handling the complete user lifecycle. Built with **Go** and following the **Hexagonal Architecture** (Ports & Adapters), it features robust, production-ready capabilities including **Bcrypt password hashing**, durable persistence via **PostgreSQL** and enforcement of the **At-Most-Once** policy using a **Redis**-backed Idempotency Key mechanism to guarantee reliable registration. `
	humaConfig.DocsPath = "/docs/user"
	humaConfig.OpenAPIPath = "/schemas/user"

	humaAPI := humago.New(mux, humaConfig)
	httpAdapter.RegisterRoutes(humaAPI)

	metricsMiddleware := middleware.NewMetricsMiddleware(monitor)
	ipMiddleware := middleware.IPMiddleware(mux)
	finalHandler := metricsMiddleware(ipMiddleware)

	log.Printf("User Manager Service running on port %s", httpPort)
	log.Printf("OpenAPI docs available at http://localhost:%s/docs", httpPort)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", httpPort),
		Handler:      finalHandler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("HTTP Server listening on port %s", httpPort)
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatalf("HTTP Server failed: %v", err)
	}
}
