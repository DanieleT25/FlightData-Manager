package kafka_consumer

import (
	"context"
	"testing"

	coreapi "github.com/DanieleT25/FlightData-Manager/microservices/alert-notifier-system/internal/application/core/api"
)

type integrationEmailClient struct {
	recipient string
	subject   string
	body      string
}

func (c *integrationEmailClient) SendEmail(_ context.Context, recipient, subject, body string) error {
	c.recipient, c.subject, c.body = recipient, subject, body
	return nil
}

func TestProcessMessageIntegratesKafkaPayloadAndNotifierService(t *testing.T) {
	email := &integrationEmailClient{}
	consumer := &KafkaConsumer{service: coreapi.NewApplication(email)}

	if err := consumer.processMessage(context.Background(), []byte(`{"user_email":"mario@example.com","airport_code":"LICC","message":"Traffic exceeded threshold"}`)); err != nil {
		t.Fatalf("processMessage() error = %v", err)
	}
	if email.recipient != "mario@example.com" || email.subject != "Alert for LICC" {
		t.Fatalf("email = %#v", email)
	}
}

func TestProcessMessageRejectsMalformedPayload(t *testing.T) {
	consumer := &KafkaConsumer{service: coreapi.NewApplication(&integrationEmailClient{})}
	if err := consumer.processMessage(context.Background(), []byte(`not-json`)); err == nil {
		t.Fatal("processMessage() accepted malformed JSON")
	}
}
