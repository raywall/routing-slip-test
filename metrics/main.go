package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	metrics "github.com/raywall/custom-business-metrics/service"
)

func main() {
	service, err := metrics.New(context.Background(), metrics.Config{
		StorageBackend: metrics.StorageDynamoDB,
		DynamoDBTable:  env("DYNAMODB_TABLE", "custom-business-metrics-events"),
		AWSRegion:      env("AWS_REGION", "us-east-1"),
		DynamoEndpoint: env("DYNAMODB_ENDPOINT", "http://localhost:4566"),
		RetentionDays:  intEnv("RETENTION_DAYS", 30),
	})
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:              env("SERVICE_ADDR", ":8080"),
		Handler:           service.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("metrics service em http://localhost%s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
