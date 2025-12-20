package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/DanieleT25/FlightData-Manager/microservices/alert-notifier-system/config"
	"github.com/DanieleT25/FlightData-Manager/microservices/alert-notifier-system/internal/adapters/email_sender"
	"github.com/DanieleT25/FlightData-Manager/microservices/alert-notifier-system/internal/adapters/kafka_consumer"
	"github.com/DanieleT25/FlightData-Manager/microservices/alert-notifier-system/internal/application/core/api"
)

func main() {
	log.Println("Starting Alert Notifier Service...")

	broker := config.GetKafkaBroker()
	topic := config.GetKafkaTopic()
	groupID := config.GetConsumerGroupID()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	emailSender := email_sender.NewLogEmailSender()

	app := api.NewApplication(emailSender)

	consumer, err := kafka_consumer.NewKafkaConsumer(broker, groupID, topic, app)
	if err != nil {
		log.Fatalf("Failed to init consumer: %v", err)
	}

	go consumer.Start(ctx)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	cancel()
	log.Println("Bye.")
}
