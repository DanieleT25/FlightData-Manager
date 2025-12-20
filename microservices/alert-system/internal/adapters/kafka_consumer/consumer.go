package kafka_consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/DanieleT25/FlightData-Manager/microservices/alert-system/internal/ports"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type KafkaConsumer struct {
	consumer *kafka.Consumer
	service  ports.AlertService
	topic    string
}

type AirportUpdateEvent struct {
	AirportCode     string `json:"airport_code"`
	TotalArrivals   int    `json:"total_arrivals"`
	TotalDepartures int    `json:"total_departures"`
	Timestamp       int64  `json:"timestamp"`
}

func NewKafkaConsumer(brokerAddr, groupID, topic string, service ports.AlertService) (*KafkaConsumer, error) {
	config := &kafka.ConfigMap{
		"bootstrap.servers":  brokerAddr,
		"group.id":           groupID,
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": "false",
	}

	c, err := kafka.NewConsumer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	err = c.SubscribeTopics([]string{topic}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
	}

	return &KafkaConsumer{
		consumer: c,
		service:  service,
		topic:    topic,
	}, nil
}

func (kc *KafkaConsumer) Start(ctx context.Context) {
	log.Printf("Starting Kafka Consumer for topic: %s", kc.topic)
	defer kc.consumer.Close()

	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, shutting down consumer...")
			return
		default:
			ev := kc.consumer.Poll(100)
			if ev == nil {
				continue
			}

			switch e := ev.(type) {
			case *kafka.Message:
				if e.TopicPartition.Error != nil {
					log.Printf("Consumer error: %v", e.TopicPartition.Error)
					continue
				}

				var updateEvent AirportUpdateEvent
				if err := json.Unmarshal(e.Value, &updateEvent); err != nil {
					log.Printf("Error decoding: %v", err)
					_, _ = kc.consumer.CommitMessage(e)
					continue
				}

				log.Printf("Processing update for %s", updateEvent.AirportCode)

				err := kc.service.CheckThresholds(ctx, updateEvent.AirportCode, updateEvent.TotalArrivals, updateEvent.TotalDepartures)
				if err != nil {
					log.Printf("Error processing logic: %v", err)
				} else {
					_, err := kc.consumer.CommitMessage(e)
					if err != nil {
						log.Printf("Commit failed: %v", err)
					}
				}

			case kafka.Error:
				if e.Code() == kafka.ErrAllBrokersDown {
					log.Printf("Critical Kafka error: %v", e)
				}
			}
		}
	}
}
