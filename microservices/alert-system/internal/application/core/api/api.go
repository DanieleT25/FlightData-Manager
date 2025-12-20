package api

import (
	"context"
	"fmt"
	"log"

	"github.com/DanieleT25/FlightData-Manager/microservices/alert-system/internal/application/core/domain"
	"github.com/DanieleT25/FlightData-Manager/microservices/alert-system/internal/ports"
)

type Application struct {
	repo     ports.InterestRepository
	producer ports.NotificationProducer
}

func NewApplication(repo ports.InterestRepository, producer ports.NotificationProducer) *Application {
	return &Application{
		repo:     repo,
		producer: producer,
	}
}

func (a *Application) CheckThresholds(ctx context.Context, airportCode string, arrivals, departures int) error {
	interests, err := a.repo.GetUsersByAirport(ctx, airportCode)
	if err != nil {
		return fmt.Errorf("failed to fetch interested users for %s: %w", airportCode, err)
	}

	if len(interests) == 0 {
		log.Printf("No users interested in %s. Skipping logic.", airportCode)
		return nil
	}

	log.Printf("Checking thresholds for %d users interested in %s (Arr: %d, Dep: %d)", len(interests), airportCode, arrivals, departures)

	totalFlights := arrivals + departures

	for _, interest := range interests {
		triggered := false
		var reason string

		if interest.LowValue != nil {
			if totalFlights < *interest.LowValue {
				triggered = true
				reason = fmt.Sprintf("Total flights (%d) dropped below LOW threshold (%d)", totalFlights, *interest.LowValue)
			}
		}

		if !triggered && interest.HighValue != nil {
			if totalFlights > *interest.HighValue {
				triggered = true
				reason = fmt.Sprintf("Total flights (%d) exceeded HIGH threshold (%d)", totalFlights, *interest.HighValue)
			}
		}

		if triggered {
			log.Printf("Threshold triggered for user %s on %s: %s", interest.UserEmail, airportCode, reason)

			notification := domain.Notification{
				UserEmail:   interest.UserEmail,
				AirportCode: airportCode,
				Message:     reason,
			}

			if err := a.producer.SendNotification(notification); err != nil {
				log.Printf("ERROR: Failed to send notification for %s: %v", interest.UserEmail, err)
			}
		}
	}

	return nil
}
