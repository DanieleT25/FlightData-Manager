package ports

import (
	"context"

	"github.com/DanieleT25/FlightData-Manager/microservices/user-manager/internal/application/core/domain"
)

type UserRepository interface {
	Save(ctx context.Context, user *domain.User) error
	Get(ctx context.Context, email string) (*domain.User, error)
	Delete(ctx context.Context, email string) error
}

type IdempotencyRepository interface {
	CheckIdempotency(ctx context.Context, clientIP string, messageID string) (bool, *domain.User, error)
	SaveIdempotencyResponse(ctx context.Context, clientIP string, messageID string, user *domain.User) error
	DeleteIdempotencyKey(ctx context.Context, clientIP, messageID string) error
}
