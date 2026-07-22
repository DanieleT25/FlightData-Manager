package kafka_consumer

import (
	"context"
	"testing"

	coreapi "github.com/DanieleT25/FlightData-Manager/microservices/alert-system/internal/application/core/api"
	"github.com/DanieleT25/FlightData-Manager/microservices/alert-system/internal/application/core/domain"
)

type integrationInterestRepository struct {
	interests []domain.UserInterest
}

func (r integrationInterestRepository) GetUsersByAirport(_ context.Context, _ string) ([]domain.UserInterest, error) {
	return r.interests, nil
}

type integrationNotificationProducer struct {
	notifications []domain.Notification
}

func (p *integrationNotificationProducer) SendNotification(notification domain.Notification) error {
	p.notifications = append(p.notifications, notification)
	return nil
}

func TestProcessMessageIntegratesKafkaPayloadAndAlertService(t *testing.T) {
	high := 10
	producer := &integrationNotificationProducer{}
	service := coreapi.NewApplication(integrationInterestRepository{interests: []domain.UserInterest{{UserEmail: "mario@example.com", HighValue: &high}}}, producer)
	consumer := &KafkaConsumer{service: service}

	if err := consumer.processMessage(context.Background(), []byte(`{"airport_code":"LICC","total_arrivals":6,"total_departures":7,"timestamp":1}`)); err != nil {
		t.Fatalf("processMessage() error = %v", err)
	}
	if len(producer.notifications) != 1 || producer.notifications[0].AirportCode != "LICC" {
		t.Fatalf("notifications = %#v", producer.notifications)
	}
}

func TestProcessMessageRejectsMalformedPayload(t *testing.T) {
	consumer := &KafkaConsumer{service: coreapi.NewApplication(integrationInterestRepository{}, &integrationNotificationProducer{})}
	if err := consumer.processMessage(context.Background(), []byte(`not-json`)); err == nil {
		t.Fatal("processMessage() accepted malformed JSON")
	}
}
