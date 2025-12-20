package config

import (
	"log"
	"os"
)

func GetNeo4jURI() string {
	return getEnvironmentValue("NEO4J_URI")
}

func GetNeo4jAuth() (string, string) {
	return getEnvironmentValue("NEO4J_USER"), getEnvironmentValue("NEO4J_PASSWORD")
}

func GetKafkaBroker() string {
	return getEnvironmentValue("KAFKA_BROKER")
}

func GetInputTopic() string {
	return getEnvironmentValue("KAFKA_TOPIC_INPUT")
}

func GetOutputTopic() string {
	return getEnvironmentValue("KAFKA_TOPIC_OUTPUT")
}

func GetConsumerGroupID() string {
	val := os.Getenv("KAFKA_GROUP_ID")
	if val == "" {
		return "alert-system-group"
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
