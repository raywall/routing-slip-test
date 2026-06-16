package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"time"

	metrics "github.com/raywall/custom-business-metrics/agent"
	routing "github.com/raywall/routing-slip-pattern/app/framework"
	"github.com/raywall/routing-slip-pattern/app/slip"
	"github.com/raywall/routing-slip-pattern/app/source"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	agent, err := metrics.New(metrics.Config{
		ServiceEndpoint: env("METRICS_ENDPOINT", "http://localhost:8080/v1/metrics"),
		BatchSize:       1,
		FlushInterval:   250 * time.Millisecond,
		Logger:          logger,
	})
	if err != nil {
		log.Fatal(err)
	}
	go func() { _ = agent.Run(ctx) }()
	defer agent.Close()

	stateStore, err := slip.NewFileStateStore(".routing-slip-state")
	if err != nil {
		log.Fatal(err)
	}
	runtime, err := routing.New(ctx, routing.Options{
		ConfigSource:   source.Local{Path: "config.yaml"},
		WorkflowSource: source.Local{Path: "workflow.yaml"},
		MetricsAgent:   agent,
		StateStore:     stateStore,
		Logger:         logger,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := runtime.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
