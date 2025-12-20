package api

import (
	"context"
	"fmt"

	"github.com/DanieleT25/FlightData-Manager/microservices/alert-notifier-system/internal/application/core/domain"
	"github.com/DanieleT25/FlightData-Manager/microservices/alert-notifier-system/internal/ports"
)

type Application struct {
	emailClient ports.EmailClient
}

func NewApplication(emailClient ports.EmailClient) *Application {
	return &Application{
		emailClient: emailClient,
	}
}

func (a *Application) NotifyUser(ctx context.Context, n domain.Notification) error {
	subject := fmt.Sprintf("Alert for %s", n.AirportCode)
	return a.emailClient.SendEmail(ctx, n.UserEmail, subject, n.Message)
}
