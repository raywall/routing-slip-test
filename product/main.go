package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	metrics "github.com/raywall/custom-business-metrics/agent"
	routing "github.com/raywall/routing-slip-pattern/app/framework"
	"github.com/raywall/routing-slip-pattern/app/slip"
	"github.com/raywall/routing-slip-pattern/app/source"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
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
	defer agent.Close()
	go func() { _ = agent.Run(ctx) }()

	runtime, err := routing.New(ctx, routing.Options{
		ConfigSource:   source.Local{Path: env("PRODUCT_CONFIG_PATH", "config.yaml")},
		WorkflowSource: source.Local{Path: env("PRODUCT_WORKFLOW_PATH", "workflow.yaml")},
		MetricsAgent:   agent,
		Logger:         logger,
	})
	if err != nil {
		log.Fatal(err)
	}

	server := startHTTPServer(ctx, runtime, logger)
	client, queueURL, err := buildSQSClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	logger.Info("product service ready",
		slog.String("queue_url", queueURL),
		slog.String("http_addr", env("PRODUCT_HTTP_ADDR", ":8087")),
		slog.String("mcp_addr", env("PRODUCT_MCP_ADDR", ":9094")),
	)

	if err := consumeQueue(ctx, client, queueURL, runtime, logger); err != nil && !errors.Is(err, context.Canceled) {
		_ = server.Shutdown(context.Background())
		log.Fatal(err)
	}
}

func startHTTPServer(ctx context.Context, runtime *routing.Runtime, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":   "ok",
			"service":  "product-service",
			"workflow": "product-processing-sqs",
		})
	})
	mux.Handle("/mcp", runtime.MCPHandler())

	server := &http.Server{
		Addr:              env("PRODUCT_HTTP_ADDR", ":8087"),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	go func() {
		logger.Info("product http endpoint listening", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("product http server stopped", slog.String("error", err.Error()))
		}
	}()

	go func() {
		mcpServer := &http.Server{
			Addr:              env("PRODUCT_MCP_ADDR", ":9094"),
			Handler:           runtime.MCPHandler(),
			ReadHeaderTimeout: 5 * time.Second,
		}
		go func() {
			<-ctx.Done()
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = mcpServer.Shutdown(shutdown)
		}()
		logger.Info("product mcp endpoint listening", slog.String("addr", mcpServer.Addr))
		if err := mcpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("product mcp server stopped", slog.String("error", err.Error()))
		}
	}()

	return server
}

func buildSQSClient(ctx context.Context) (*sqs.Client, string, error) {
	endpoint := env("PRODUCT_SQS_ENDPOINT", "http://localhost:4566")
	region := env("PRODUCT_AWS_REGION", "us-east-1")
	queueURL := env("PRODUCT_SQS_QUEUE_URL", "http://localhost:4566/000000000000/sample-test-convenio-133341")
	cfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithBaseEndpoint(endpoint),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			env("PRODUCT_AWS_ACCESS_KEY_ID", "test"),
			env("PRODUCT_AWS_SECRET_ACCESS_KEY", "test"),
			"",
		)),
	)
	if err != nil {
		return nil, "", err
	}
	return sqs.NewFromConfig(cfg), queueURL, nil
}

func consumeQueue(ctx context.Context, client *sqs.Client, queueURL string, runtime *routing.Runtime, logger *slog.Logger) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		output, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:              &queueURL,
			MaxNumberOfMessages:   1,
			WaitTimeSeconds:       10,
			VisibilityTimeout:     30,
			MessageAttributeNames: []string{"All"},
		})
		if err != nil {
			logger.Error("receive sqs message failed", slog.String("error", err.Error()))
			time.Sleep(2 * time.Second)
			continue
		}

		if len(output.Messages) == 0 {
			continue
		}

		for _, message := range output.Messages {
			if message.Body == nil {
				continue
			}
			payload, err := decodePayload(*message.Body)
			if err != nil {
				logger.Error("invalid sqs payload",
					slog.String("message_id", safeString(message.MessageId)),
					slog.String("error", err.Error()),
				)
				continue
			}

			result, runErr := runtime.Process(ctx, payload)
			if runErr != nil {
				logger.Error("workflow execution failed",
					slog.String("message_id", safeString(message.MessageId)),
					slog.String("workflow_message_id", workflowMessageID(result)),
					slog.String("error", runErr.Error()),
				)
				continue
			}

			logger.Info("workflow execution completed",
				slog.String("message_id", workflowMessageID(result)),
				slog.String("correlation_id", workflowCorrelationID(result)),
			)

			if message.ReceiptHandle != nil {
				if _, err := client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
					QueueUrl:      &queueURL,
					ReceiptHandle: message.ReceiptHandle,
				}); err != nil {
					logger.Error("delete sqs message failed",
						slog.String("message_id", safeString(message.MessageId)),
						slog.String("error", err.Error()),
					)
				}
			}
		}
	}
}

func decodePayload(body string) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func workflowMessageID(msg *slip.Message) string {
	if msg == nil {
		return ""
	}
	return msg.ID
}

func workflowCorrelationID(msg *slip.Message) string {
	if msg == nil {
		return ""
	}
	return msg.CorrelationID
}

func safeString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
