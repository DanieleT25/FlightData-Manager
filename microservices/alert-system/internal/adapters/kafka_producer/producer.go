package kafka_producer

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/DanieleT25/FlightData-Manager/microservices/alert-system/internal/application/core/domain"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type KafkaProducer struct {
	producer *kafka.Producer
	topic    string
}

func deliveryReportHandler(events chan kafka.Event) {
	for e := range events {
		switch ev := e.(type) {
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				log.Printf("Delivery failed: %v\n", ev.TopicPartition.Error)
			} else {
				log.Printf("Message delivered to %s [%d] at offset %v\n",
					*ev.TopicPartition.Topic, ev.TopicPartition.Partition, ev.TopicPartition.Offset)
			}
		}
	}
}

func NewKafkaProducer(brokerAddr, topic string) (*KafkaProducer, error) {
	config := &kafka.ConfigMap{
		"bootstrap.servers":                     brokerAddr,
		"acks":                                  "all",
		"retries":                               3,
		"linger.ms":                             10,
		"max.in.flight.requests.per.connection": 1,
	}

	producer, err := kafka.NewProducer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kafka producer: %w", err)
	}

	go deliveryReportHandler(producer.Events())

	log.Printf("Kafka Producer created for topic: %s", topic)

	return &KafkaProducer{
		producer: producer,
		topic:    topic,
	}, nil
}

func (kp *KafkaProducer) SendNotification(alert domain.Notification) error {
	payload, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %w", err)
	}

	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &kp.topic, Partition: kafka.PartitionAny},
		Key:            []byte(alert.UserEmail),
		Value:          payload,
	}

	err = kp.producer.Produce(msg, nil)
	if err != nil {
		return fmt.Errorf("failed to enqueue message: %w", err)
	}

	return nil
}

func (kp *KafkaProducer) Close() {
	log.Println("Flushing remaining messages...")
	unflushed := kp.producer.Flush(10000)
	if unflushed > 0 {
		log.Printf("Warning: %d messages were not flushed\n", unflushed)
	} else {
		log.Println("All messages delivered successfully")
	}
	kp.producer.Close()
}
