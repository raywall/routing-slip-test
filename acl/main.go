package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/raywall/cloud-easy-connector/pkg/cloud"
	metrics "github.com/raywall/custom-business-metrics/agent"
	"github.com/raywall/go-graphql-connector/graphql"
	"github.com/segmentio/kafka-go"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	agent, err := metrics.New(metrics.Config{
		ServiceEndpoint: env("METRICS_ENDPOINT", "http://localhost:8080/v1/metrics"),
		BatchSize:       1,
		FlushInterval:   250 * time.Millisecond,
	})
	if err != nil {
		log.Fatal(err)
	}
	go func() { _ = agent.Run(ctx) }()
	defer agent.Close()
	go consumeKafka(ctx)

	config, err := graphql.LoadConfigFrom(ctx, graphql.ReferenceSource{
		Reference: env("GRAPHQL_CONFIG_REFERENCE", "local:config/service.json"),
		Region:    "us-east-1",
	})
	if err != nil {
		log.Fatal(err)
	}
	resources := &cloud.CloudContextList{cloud.SSMContext, cloud.SecretsManagerContext}
	api, err := graphql.NewWithOptions(config, resources, env("AWS_REGION", "us-east-1"), env("AWS_ENDPOINT_URL", ""), graphql.Options{MetricsEmitter: agent})
	if err != nil {
		log.Fatal(err)
	}
	if err := api.Serve(ctx, graphql.ServerOptions{Addr: env("GRAPHQL_ADDR", ":8090"), MCPAddr: env("GRAPHQL_MCP_ADDR", ":9092")}); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}

func consumeKafka(ctx context.Context) {
	brokers := splitEnv("ACL_KAFKA_BROKERS")
	topic := env("ACL_KAFKA_TOPIC", "")
	if len(brokers) == 0 || topic == "" {
		return
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: env("ACL_KAFKA_GROUP_ID", "sample-test-acl"),
	})
	defer reader.Close()
	log.Printf("acl kafka consumer started topic=%s brokers=%s", topic, strings.Join(brokers, ","))
	for {
		message, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("acl kafka consumer error: %v", err)
			time.Sleep(time.Second)
			continue
		}
		log.Printf("acl consumed kafka event topic=%s partition=%d offset=%d key=%s bytes=%d", message.Topic, message.Partition, message.Offset, string(message.Key), len(message.Value))
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func splitEnv(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}
