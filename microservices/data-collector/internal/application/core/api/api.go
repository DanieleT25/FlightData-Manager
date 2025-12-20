package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/apperrors"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/application/core/domain"
	"github.com/DanieleT25/FlightData-Manager/microservices/data-collector/internal/ports"
)

type Application struct {
	db         ports.FlightRepository
	userClient ports.UserManagerClient
	openSky    ports.OpenSkyClient
	producer   ports.EventProducer
	interval   time.Duration
}

func NewApplication(db ports.FlightRepository, userClient ports.UserManagerClient, openSky ports.OpenSkyClient, producer ports.EventProducer, interval time.Duration) *Application {
	return &Application{
		db:         db,
		userClient: userClient,
		openSky:    openSky,
		producer:   producer,
		interval:   interval,
	}
}

func (a *Application) SetUserInterests(ctx context.Context, email, password string, interests []domain.Interest) error {
	if len(interests) == 0 {
		return fmt.Errorf("%w: interests list cannot be empty", apperrors.ErrInvalidInput)
	}

	if err := a.verifyUserCredentials(ctx, email, password); err != nil {
		return err
	}

	if err := a.db.SetInterests(ctx, email, interests); err != nil {
		return fmt.Errorf("%w: %v", apperrors.ErrDbOperation, err)
	}

	return nil
}

func (a *Application) GetUserInterests(ctx context.Context, email, password string) ([]domain.Interest, error) {
	if err := a.verifyUserCredentials(ctx, email, password); err != nil {
		return nil, err
	}

	return a.db.GetInterests(ctx, email)
}

func (a *Application) GetAirportFlights(ctx context.Context, email, password, airportCode string, limit int) ([]domain.Flight, error) {
	if err := a.checkAccessWithPassword(ctx, email, password, airportCode); err != nil {
		return nil, err
	}

	return a.db.GetFlights(ctx, airportCode, limit)
}

func (a *Application) GetLastFlight(ctx context.Context, email, password, airportCode, direction string) (*domain.Flight, error) {
	if err := a.checkAccessWithPassword(ctx, email, password, airportCode); err != nil {
		return nil, err
	}

	flight, err := a.db.GetLastFlight(ctx, airportCode, direction)
	if err != nil {
		if errors.Is(err, apperrors.ErrNoDataFound) {
			return nil, apperrors.ErrNoDataFound
		}
		return nil, err
	}
	return flight, nil
}

func (a *Application) GetFlightsAverage(ctx context.Context, email, password, airportCode, direction string, days int) (float64, error) {
	if days <= 0 || days > 365 {
		return 0, fmt.Errorf("%w: days must be positive and max 365", apperrors.ErrInvalidInput)
	}

	if err := a.checkAccessWithPassword(ctx, email, password, airportCode); err != nil {
		return 0, err
	}

	_, err := a.db.GetLastFlight(ctx, airportCode, direction)
	if err != nil {
		if errors.Is(err, apperrors.ErrNoDataFound) {
			return 0, apperrors.ErrNoDataFound
		}
		return 0, err
	}

	duration := time.Duration(days) * 24 * time.Hour
	startTime := time.Now().Add(-duration).Unix()

	totalFlights, err := a.db.GetFlightsCount(ctx, airportCode, direction, startTime)
	if err != nil {
		return 0, err
	}

	average := float64(totalFlights) / float64(days)

	return average, nil
}
