package ports

import (
	"context"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/domain"
)

type CollectorAPI interface {
	SetUserInterests(ctx context.Context, email, password string, airportCodes []string) error
	GetUserInterests(ctx context.Context, email, password string) ([]string, error)
	GetAirportFlights(ctx context.Context, email, password, airportCode string, limit int) ([]domain.Flight, error)
	GetLastFlight(ctx context.Context, email, password, airportCode, direction string) (*domain.Flight, error)
	GetFlightsAverage(ctx context.Context, email, password, airportCode, direction string, days int) (float64, error)
	RunCollectionCycle(ctx context.Context)
}
