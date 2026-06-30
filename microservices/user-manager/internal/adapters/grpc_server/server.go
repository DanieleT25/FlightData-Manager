package grpc_server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/adapters/observability"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/apperrors"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/ports"
	pb "github.com/DanieleT25/FlightData-Manager/pkg/proto/user"
	"golang.org/x/crypto/bcrypt"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

type GrpcAdapter struct {
	app     ports.UserAPI
	monitor *observability.Monitor
	pb.UnimplementedUserServiceServer
}

func NewGrpcAdapter(app ports.UserAPI, monitor *observability.Monitor) *GrpcAdapter {
	return &GrpcAdapter{app: app, monitor: monitor}
}

func (g *GrpcAdapter) VerifyCredentials(ctx context.Context, req *pb.VerifyCredentialsRequest) (*pb.VerifyCredentialsResponse, error) {
	if req.Email == "" || req.Password == "" {
		st := status.New(codes.InvalidArgument, "invalid credentials request")
		br := &errdetails.BadRequest{}

		if req.Email == "" {
			br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
				Field: "email", Description: "email cannot be empty",
			})
		}
		if req.Password == "" {
			br.FieldViolations = append(br.FieldViolations, &errdetails.BadRequest_FieldViolation{
				Field: "password", Description: "password cannot be empty",
			})
		}

		stWithDetails, _ := st.WithDetails(br)
		return nil, stWithDetails.Err()
	}

	user, err := g.app.GetUser(ctx, req.Email)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			return &pb.VerifyCredentialsResponse{Valid: false}, nil
		}
		return nil, status.Errorf(codes.Unavailable, "internal storage unavailable: %v", err)
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return &pb.VerifyCredentialsResponse{Valid: false}, nil
	}

	return &pb.VerifyCredentialsResponse{Valid: true}, nil
}

func (g *GrpcAdapter) CheckUserExistence(ctx context.Context, req *pb.UserExistenceRequest) (*pb.UserExistenceResponse, error) {
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email cannot be empty")
	}

	_, err := g.app.GetUser(ctx, req.Email)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			return &pb.UserExistenceResponse{
				Exists: false,
			}, nil
		}
		return nil, status.Errorf(codes.Unavailable, "internal storage unavailable: %v", err)
	}

	return &pb.UserExistenceResponse{
		Exists: true,
	}, nil
}

func (g *GrpcAdapter) Register(grpcServer *grpc.Server) {
	pb.RegisterUserServiceServer(grpcServer, g)
}

func (g *GrpcAdapter) Start(port string) error {
	tlsCreds, err := loadServerTLSCredentials()
	if err != nil {
		return fmt.Errorf("failed to load TLS credentials: %w", err)
	}

	listenAddr := fmt.Sprintf(":%s", port)
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	opts := []grpc.ServerOption{
		grpc.Creds(tlsCreds),
		grpc.UnaryInterceptor(g.metricsInterceptor),
	}

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterUserServiceServer(grpcServer, g)

	log.Printf("gRPC Server listening on port %s", port)

	return grpcServer.Serve(lis)
}

func loadServerTLSCredentials() (credentials.TransportCredentials, error) {
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

func (g *GrpcAdapter) metricsInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	g.monitor.IncInFlight()
	defer g.monitor.DecInFlight()

	start := time.Now()
	resp, err := handler(ctx, req)
	duration := time.Since(start).Seconds()
	st, _ := status.FromError(err)
	statusCode := st.Code().String()

	g.monitor.ObserveDuration("gRPC", info.FullMethod, duration)
	g.monitor.IncRequest("gRPC", info.FullMethod, statusCode)

	return resp, err
}
