package config

import (
	"log"
	"os"
	"strconv"
)

func GetRedisAddr() string {
	return getEnvironmentValue("REDIS_ADDR")
}

func GetRedisPassword() string {
	return os.Getenv("REDIS_PASSWORD")
}

func GetRedisDB() int {
	val := os.Getenv("REDIS_DB")
	if val == "" {
		return 0
	}

	dbIndex, err := strconv.Atoi(val)
	if err != nil {
		log.Fatalf("CRITICAL: REDIS_DB must be an integer, got: %s", val)
	}
	return dbIndex
}

func GetServerPort() string {
	return getEnvironmentValue("SERVER_PORT")
}

func GetGRPCPort() string {
	return getEnvironmentValue("GRPC_PORT")
}

func GetServiceName() string {
	return getEnvironmentValue("SERVICE_NAME")
}

func GetNodeName() string {
	return getEnvironmentValue("NODE_NAME")
}

func getEnvironmentValue(key string) string {
	if os.Getenv(key) == "" {
		log.Fatalf("%s environment variable is missing.", key)
	}
	return os.Getenv(key)
}
