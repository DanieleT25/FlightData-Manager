package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/DanieleT25/FlightData-Manager/microservices/alert-system/config"
	"github.com/DanieleT25/FlightData-Manager/microservices/alert-system/internal/adapters/kafka_consumer"
	"github.com/DanieleT25/FlightData-Manager/microservices/alert-system/internal/adapters/kafka_producer"
	"github.com/DanieleT25/FlightData-Manager/microservices/alert-system/internal/adapters/repository"
	"github.com/DanieleT25/FlightData-Manager/microservices/alert-system/internal/application/core/api"
)

func main() {
	log.Println("Starting Alert System Service...")

	neo4jURI := config.GetNeo4jURI()
	neoUser, neoPass := config.GetNeo4jAuth()
	kafkaBroker := config.GetKafkaBroker()
	inputTopic := config.GetInputTopic()
	outputTopic := config.GetOutputTopic()
	groupID := config.GetConsumerGroupID()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("Connecting to Neo4j at %s...", neo4jURI)
	repo, err := repository.NewNeo4jRepository(ctx, neo4jURI, neoUser, neoPass)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to connect to Neo4j: %v", err)
	}
	defer repo.Close(ctx)
	log.Println("Neo4j connected successfully.")

	log.Printf("Initializing Kafka Producer for topic '%s'...", outputTopic)
	producer, err := kafka_producer.NewKafkaProducer(kafkaBroker, outputTopic)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to create Kafka producer: %v", err)
	}
	defer producer.Close()
	log.Println("Kafka Producer ready.")

	alertApp := api.NewApplication(repo, producer)

	log.Printf("Initializing Kafka Consumer (Group: %s, Topic: %s)...", groupID, inputTopic)
	consumer, err := kafka_consumer.NewKafkaConsumer(kafkaBroker, groupID, inputTopic, alertApp)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to create Kafka consumer: %v", err)
	}

	go consumer.Start(ctx)

	log.Println("Alert System is running and listening for events...")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("Received termination signal: %v. Shutting down...", sig)

	cancel()

	log.Println("Shutdown complete.")
}
