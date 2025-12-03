package ports

import (
	"context"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/domain"
)

type FlightRepository interface {
	SetInterests(ctx context.Context, email string, airportCodes []string) error
	GetInterests(ctx context.Context, email string) ([]string, error)
	IsUserInterested(ctx context.Context, email string, airportCode string) (bool, error)
	GetFlights(ctx context.Context, airportCode string, limit int) ([]domain.Flight, error)
	GetLastFlight(ctx context.Context, airportCode string, direction string) (*domain.Flight, error)
	GetFlightsCount(ctx context.Context, airportCode string, direction string, startTime int64) (int64, error)
	SetFlight(ctx context.Context, flight *domain.Flight) error
	GetAllUsers(ctx context.Context) ([]string, error)
	DeleteUserNodes(ctx context.Context, email string) error
	GetAirportsToMonitor(ctx context.Context) (map[string]int64, error)
	UpdateAirportLastSync(ctx context.Context, airportCode string, timestamp int64) error
}
