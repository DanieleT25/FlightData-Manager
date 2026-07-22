package grpc_server

import (
	"context"
	"net"
	"testing"

	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/adapters/observability"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/apperrors"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/domain"
	pb "github.com/DanieleT25/FlightData-Manager/pkg/proto/user"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type grpcUserApplication struct {
	user *domain.User
}

func (a grpcUserApplication) RegisterUser(context.Context, string, string, string, string, string, string, string) (*domain.User, error) {
	return nil, nil
}
func (a grpcUserApplication) GetUser(_ context.Context, email string) (*domain.User, error) {
	if a.user == nil || a.user.Email != email {
		return nil, apperrors.ErrUserNotFound
	}
	return a.user, nil
}
func (grpcUserApplication) DeleteUser(context.Context, string, string) error { return nil }
func (grpcUserApplication) CheckIdempotencyUser(context.Context, string, string) (bool, *domain.User, error) {
	return true, nil, nil
}
func (grpcUserApplication) SaveIdempotencyResponseUser(context.Context, string, string, *domain.User) error {
	return nil
}
func (grpcUserApplication) DeleteIdempotencyKeyUser(context.Context, string, string) error {
	return nil
}

func TestVerifyCredentialsOverGRPC(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("Password123!"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	adapter := NewGrpcAdapter(grpcUserApplication{user: &domain.User{Email: "mario@example.com", PasswordHash: string(hash)}}, observability.NewMonitor("test", "test-node"))
	adapter.Register(server)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := pb.NewUserServiceClient(conn)
	response, err := client.VerifyCredentials(context.Background(), &pb.VerifyCredentialsRequest{Email: "mario@example.com", Password: "Password123!"})
	if err != nil || !response.Valid {
		t.Fatalf("VerifyCredentials() response = %#v, error = %v", response, err)
	}

	response, err = client.VerifyCredentials(context.Background(), &pb.VerifyCredentialsRequest{Email: "mario@example.com", Password: "wrong"})
	if err != nil || response.Valid {
		t.Fatalf("VerifyCredentials() wrong password response = %#v, error = %v", response, err)
	}
}
