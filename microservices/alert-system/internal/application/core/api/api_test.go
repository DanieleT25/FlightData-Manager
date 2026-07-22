package api

import (
	"context"
	"testing"

	"github.com/DanieleT25/FlightData-Manager/microservices/alert-system/internal/application/core/domain"
)

type fakeInterestRepository struct {
	interests []domain.UserInterest
}

func (r fakeInterestRepository) GetUsersByAirport(_ context.Context, _ string) ([]domain.UserInterest, error) {
	return r.interests, nil
}

type fakeNotificationProducer struct {
	notifications []domain.Notification
}

func (p *fakeNotificationProducer) SendNotification(notification domain.Notification) error {
	p.notifications = append(p.notifications, notification)
	return nil
}

func TestCheckThresholdsSendsOnlyTriggeredNotifications(t *testing.T) {
	low, high := 5, 20
	producer := &fakeNotificationProducer{}
	app := NewApplication(fakeInterestRepository{interests: []domain.UserInterest{
		{UserEmail: "low@example.com", LowValue: &low},
		{UserEmail: "high@example.com", HighValue: &high},
	}}, producer)

	if err := app.CheckThresholds(context.Background(), "LICC", 2, 1); err != nil {
		t.Fatalf("CheckThresholds() error = %v", err)
	}
	if len(producer.notifications) != 1 || producer.notifications[0].UserEmail != "low@example.com" {
		t.Fatalf("notifications = %#v", producer.notifications)
	}
}

func TestCheckThresholdsDoesNothingWithoutInterests(t *testing.T) {
	producer := &fakeNotificationProducer{}
	app := NewApplication(fakeInterestRepository{}, producer)

	if err := app.CheckThresholds(context.Background(), "LICC", 20, 20); err != nil {
		t.Fatalf("CheckThresholds() error = %v", err)
	}
	if len(producer.notifications) != 0 {
		t.Fatalf("notifications = %#v, want none", producer.notifications)
	}
}
