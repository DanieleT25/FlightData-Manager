package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"google.golang.org/grpc/credentials"

	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/config"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/adapters/grpc_server"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/adapters/huma_api"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/adapters/repository"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/api"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/middleware"
	"google.golang.org/grpc"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
)

func main() {
	redisAddr := config.GetRedisAddr()
	redisPass := config.GetRedisPassword()
	redisDB := config.GetRedisDB()
	httpPort := config.GetServerPort()
	grpcPort := config.GetGRPCPort()
	env := config.GetEnv()

	ctx := context.Background()

	log.Printf("Starting User Manager Service (Environment: %s)", env)
	log.Printf("Connecting to Redis at %s (DB: %d)...", redisAddr, redisDB)

	userRepo, err := repository.NewRedisRepository(ctx, redisAddr, redisPass, redisDB)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis successfully.")

	userApplication := api.NewApplication(userRepo)

	httpAdapter := huma_api.NewAPIHandler(userApplication)
	grpcAdapter := grpc_server.NewGrpcAdapter(userApplication)

	go func() {
		tlsCreds, err := loadTLSCredentials()
		if err != nil {
			log.Fatalf("Failed to load TLS keys: %v", err)
		}

		listenAddr := fmt.Sprintf(":%s", grpcPort)
		lis, err := net.Listen("tcp", listenAddr)
		if err != nil {
			log.Fatalf("Failed to listen for gRPC: %v", err)
		}

		grpcServer := grpc.NewServer(grpc.Creds(tlsCreds))
		grpcAdapter.Register(grpcServer)

		log.Printf("gRPC Server listening on port %s", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	mux := http.NewServeMux()

	humaConfig := huma.DefaultConfig("User Manager API", "1.0.0")
	humaConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{}
	humaConfig.Info.Description = `This **User Manager Microservice** is a core component of the distributed flight monitoring platform, responsible for handling the complete user lifecycle. Built with **Go** and following the **Hexagonal Architecture** (Ports & Adapters), it features robust, production-ready capabilities including **Bcrypt password hashing**, secure persistence via **Redis** and enforcement of the **At-Most-Once** policy using an Idempotency Key mechanism to guarantee reliable registration. `
	humaConfig.DocsPath = "/docs/user"
	humaConfig.OpenAPIPath = "/schemas/user"

	humaAPI := humago.New(mux, humaConfig)
	httpAdapter.RegisterRoutes(humaAPI)
	handlerConIP := middleware.IPMiddleware(mux)

	log.Printf("User Manager Service running on port %s", httpPort)
	log.Printf("OpenAPI docs available at http://localhost:%s/docs", httpPort)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%s", httpPort),
		Handler:      handlerConIP,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("HTTP Server listening on port %s", httpPort)
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatalf("HTTP Server failed: %v", err)
	}
}

func loadTLSCredentials() (credentials.TransportCredentials, error) {
	serverCert, err := tls.LoadX509KeyPair("/certs/server-cert.pem", "/certs/server-key.pem")
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	ca, err := os.ReadFile("/certs/ca-cert.pem")
	if err != nil {
		return nil, err
	}
	if !certPool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("failed to append CA cert")
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}

	return credentials.NewTLS(config), nil
}
