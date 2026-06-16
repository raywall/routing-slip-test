package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpserver "github.com/raywall/workflows/sample-test/mock-service/internal/http"
	"github.com/raywall/workflows/sample-test/mock-service/internal/store"
)

func main() {
	logger := log.New(os.Stdout, "mock-service ", log.LstdFlags|log.Lmicroseconds)
	addr := env("MOCK_SERVICE_ADDR", ":8079")
	dataPath := env("MOCK_SERVICE_DATA", "/data/mocks.json")

	repo, err := store.NewFileRepository(dataPath)
	if err != nil {
		logger.Fatalf("repository init failed: %v", err)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           httpserver.New(repo, logger).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Printf("mock service listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpserver.Shutdown(ctx, server); err != nil {
		logger.Printf("shutdown failed: %v", err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
