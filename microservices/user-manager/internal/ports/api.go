package ports

import (
	"context"

	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/domain"
)

type UserAPI interface {
	RegisterUser(ctx context.Context, email, password, firstName, lastName, cardNum, expDate, cvv string) (*domain.User, error)
	GetUser(ctx context.Context, email string) (*domain.User, error)
	DeleteUser(ctx context.Context, email string, password string) error
	CheckIdempotencyUser(ctx context.Context, ip, msgID string) (bool, *domain.User, error)
	SaveIdempotencyResponseUser(ctx context.Context, ip, msgID string, user *domain.User) error
	DeleteIdempotencyKeyUser(ctx context.Context, clientIP, messageID string) error
}
