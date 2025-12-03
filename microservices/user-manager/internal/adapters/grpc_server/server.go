package grpc_server

import (
	"context"
	"errors"

	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/apperrors"
	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/ports"
	pb "github.com/DanieleT25/FlightData-Manager/pkg/proto/user"
	"golang.org/x/crypto/bcrypt"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GrpcAdapter struct {
	app ports.UserAPI
	pb.UnimplementedUserServiceServer
}

func NewGrpcAdapter(app ports.UserAPI) *GrpcAdapter {
	return &GrpcAdapter{app: app}
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
