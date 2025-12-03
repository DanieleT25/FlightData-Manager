package ports

import (
	"context"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/domain"
)

type UserManagerClient interface {
	CheckUserExistence(ctx context.Context, email string) (bool, error)
	VerifyCredentials(ctx context.Context, email, password string) (bool, error)
}

type OpenSkyClient interface {
	GetArrivals(ctx context.Context, airport string, begin, end int64) ([]domain.Flight, error)
	GetDepartures(ctx context.Context, airport string, begin, end int64) ([]domain.Flight, error)
}
