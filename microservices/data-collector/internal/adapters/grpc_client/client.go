package grpc_client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/adapters/observability"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/apperrors"
	pb "github.com/DanieleT25/FlightData-Manager/pkg/proto/user"
	"github.com/sony/gobreaker"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
)

type UserClientAdapter struct {
	client pb.UserServiceClient
}

func NewUserClientAdapter(serverAddr string, monitor *observability.Monitor) (*UserClientAdapter, error) {
	retryOpts := []grpc_retry.CallOption{
		grpc_retry.WithMax(3),
		grpc_retry.WithCodes(codes.Unavailable, codes.ResourceExhausted),
		grpc_retry.WithBackoff(grpc_retry.BackoffLinear(100 * time.Millisecond)),
	}

	cbSettings := gobreaker.Settings{
		Name:        "user-manager",
		MaxRequests: 3,
		Interval:    0,
		Timeout:     5 * time.Second,

		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 3
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			fmt.Printf("Circuit Breaker '%s' changed from %s to %s\n", name, from, to)

			var stateCode float64
			switch to {
			case gobreaker.StateClosed:
				stateCode = 0
			case gobreaker.StateHalfOpen:
				stateCode = 1
			case gobreaker.StateOpen:
				stateCode = 2
			}
			monitor.SetCBState("user_manager", stateCode)
		},
	}
	cb := gobreaker.NewCircuitBreaker(cbSettings)

	tlsCreds, err := loadClientTLSCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS creds: %w", err)
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithChainUnaryInterceptor(
			circuitBreakerClientInterceptor(cb, monitor),
			grpc_retry.UnaryClientInterceptor(retryOpts...),
		),
	}

	conn, err := grpc.NewClient(serverAddr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to user-manager: %w", err)
	}

	return &UserClientAdapter{client: pb.NewUserServiceClient(conn)}, nil
}

func (a *UserClientAdapter) VerifyCredentials(ctx context.Context, email, password string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req := &pb.VerifyCredentialsRequest{
		Email:    email,
		Password: password,
	}

	resp, err := a.client.VerifyCredentials(ctx, req)
	if err != nil {
		return false, handleGrpcError(err)
	}

	return resp.Valid, nil
}

func (a *UserClientAdapter) CheckUserExistence(ctx context.Context, email string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req := &pb.UserExistenceRequest{Email: email}

	resp, err := a.client.CheckUserExistence(ctx, req)
	if err != nil {
		return false, handleGrpcError(err)
	}

	return resp.Exists, nil
}

func handleGrpcError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("%w: %v", apperrors.ErrExternalService, err)
	}

	for _, detail := range st.Details() {
		if t, ok := detail.(*errdetails.BadRequest); ok {
			return fmt.Errorf("%w: validation failed on %v", apperrors.ErrInvalidInput, t.FieldViolations)
		}
	}

	switch st.Code() {
	case codes.InvalidArgument:
		return fmt.Errorf("%w: remote validation failed - %s", apperrors.ErrInvalidInput, st.Message())

	case codes.Unavailable, codes.Internal:
		return fmt.Errorf("%w: user manager unavailable - %s", apperrors.ErrExternalService, st.Message())

	default:
		return fmt.Errorf("%w: grpc error %s - %s", apperrors.ErrExternalService, st.Code(), st.Message())
	}
}

func loadClientTLSCredentials() (credentials.TransportCredentials, error) {
	clientCert, err := tls.LoadX509KeyPair("/certs/client-cert.pem", "/certs/client-key.pem")
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
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
		ServerName:   "user-manager",
	}

	return credentials.NewTLS(config), nil
}
