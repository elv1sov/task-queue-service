package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task-queue-service/internal/config"
	"task-queue-service/internal/repository"
	"task-queue-service/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := repository.Open(startup, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect PostgreSQL: %w", err)
	}
	defer db.Close()
	cancel()

	slog.Info("worker started", "concurrency", cfg.WorkerConcurrency)
	service.Worker{Store: db}.Run(ctx, cfg.WorkerConcurrency)
	slog.Info("worker stopped")
	return nil
}
