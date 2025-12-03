package config

import (
	"log"
	"os"
	"time"
)

func GetServerPort() string {
	return getEnvironmentValue("SERVER_PORT")
}

func GetEnv() string {
	val := os.Getenv("ENV")
	if val == "" {
		return "development"
	}
	return val
}

func GetNeo4jURI() string {
	return getEnvironmentValue("NEO4J_URI")
}

func GetNeo4jAuth() (string, string) {
	return getEnvironmentValue("NEO4J_USER"), getEnvironmentValue("NEO4J_PASSWORD")
}

func GetUserManagerHost() string {
	return getEnvironmentValue("USER_MANAGER_GRPC_HOST")
}

func GetOpenSkyAuth() (string, string) {
	return os.Getenv("OPENSKY_USER"), os.Getenv("OPENSKY_PASSWORD")
}

func getEnvironmentValue(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("CRITICAL: %s environment variable is missing.", key)
	}
	return val
}

func GetCollectionInterval() time.Duration {
	val := os.Getenv("COLLECTION_INTERVAL")
	if val == "" {
		return 12 * time.Hour
	}

	duration, err := time.ParseDuration(val)
	if err != nil {
		log.Printf("Invalid COLLECTION_INTERVAL format (%s), defaulting to 12h", val)
		return 12 * time.Hour
	}
	return duration
}
