package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"

	metrics "github.com/raywall/custom-business-metrics/agent"
	routing "github.com/raywall/routing-slip-pattern/app/framework"
	"github.com/raywall/routing-slip-pattern/app/source"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	agent, err := metrics.New(metrics.Config{ServiceEndpoint: env("METRICS_ENDPOINT", "http://localhost:8080/v1/metrics")})
	if err != nil {
		log.Fatal(err)
	}
	go func() { _ = agent.Run(ctx) }()
	defer agent.Close()

	runtime, err := routing.New(ctx, routing.Options{
		ConfigSource:   source.Local{Path: env("ROUTING_CONFIG_PATH", "config.yaml")},
		WorkflowSource: source.Local{Path: env("ROUTING_WORKFLOW_PATH", "workflow.yaml")},
		MetricsAgent:   agent,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := runtime.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
