package ports

import (
	"context"

	"github.com/DanieleT25/FlightData-Manager/microservices/alert-notifier-system/internal/application/core/domain"
)

type NotifierService interface {
	NotifyUser(ctx context.Context, notification domain.Notification) error
}

type EmailClient interface {
	SendEmail(ctx context.Context, recipient, subject, body string) error
}
