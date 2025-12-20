package config

import (
	"log"
	"os"
)

func GetKafkaBroker() string {
	return getEnvironmentValue("KAFKA_BROKER")
}

func GetKafkaTopic() string {
	return getEnvironmentValue("KAFKA_TOPIC")
}

func GetConsumerGroupID() string {
	val := os.Getenv("KAFKA_GROUP_ID")
	if val == "" {
		return "alert-notifier-system-group"
	}
	return val
}

func getEnvironmentValue(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("CRITICAL: %s environment variable is missing.", key)
	}
	return val
}
