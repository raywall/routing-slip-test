package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	metrics "github.com/raywall/custom-business-metrics/agent"
	routing "github.com/raywall/routing-slip-pattern/app/framework"
	"github.com/raywall/routing-slip-pattern/app/source"
)

var runtime *routing.Runtime

func init() {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	agent, err := metrics.New(metrics.Config{
		ServiceEndpoint: env("METRICS_ENDPOINT", "http://metrics-service:8080/v1/metrics"),
		BatchSize:       1,
		FlushInterval:   250 * time.Millisecond,
		Logger:          logger,
	})
	if err != nil {
		log.Fatal(err)
	}
	go func() { _ = agent.Run(ctx) }()

	runtime, err = routing.New(ctx, routing.Options{
		ConfigSource:   source.Local{Path: env("ROUTING_CONFIG_PATH", "/var/task/config.yaml")},
		WorkflowSource: source.Local{Path: env("ROUTING_WORKFLOW_PATH", "/var/task/workflow.yaml")},
		MetricsAgent:   agent,
		Logger:         logger,
	})
	if err != nil {
		log.Fatal(err)
	}
}

func handler(ctx context.Context, payload map[string]any) (any, error) {
	return runtime.Process(ctx, payload)
}

func main() {
	lambda.Start(handler)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
