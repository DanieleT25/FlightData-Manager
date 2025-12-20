package ports

import (
	"context"

	"github.com/DanieleT25/FlightData-Manager/microservices/alert-system/internal/application/core/domain"
)

type AlertService interface {
	CheckThresholds(ctx context.Context, airportCode string, arrivals, departures int) error
}

type InterestRepository interface {
	GetUsersByAirport(ctx context.Context, airportCode string) ([]domain.UserInterest, error)
}

type NotificationProducer interface {
	SendNotification(notification domain.Notification) error
}
