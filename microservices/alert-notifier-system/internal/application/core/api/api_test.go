package api

import (
	"context"
	"testing"

	"github.com/DanieleT25/FlightData-Manager/microservices/alert-notifier-system/internal/application/core/domain"
)

type fakeEmailClient struct {
	recipient string
	subject   string
	body      string
}

func (c *fakeEmailClient) SendEmail(_ context.Context, recipient, subject, body string) error {
	c.recipient, c.subject, c.body = recipient, subject, body
	return nil
}

func TestNotifyUserBuildsExpectedEmail(t *testing.T) {
	email := &fakeEmailClient{}
	app := NewApplication(email)
	notification := domain.Notification{UserEmail: "mario@example.com", AirportCode: "LICC", Message: "Traffic exceeded threshold"}

	if err := app.NotifyUser(context.Background(), notification); err != nil {
		t.Fatalf("NotifyUser() error = %v", err)
	}
	if email.recipient != notification.UserEmail || email.subject != "Alert for LICC" || email.body != notification.Message {
		t.Fatalf("email = %#v", email)
	}
}
