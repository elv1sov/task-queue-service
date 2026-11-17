package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"task-queue-service/internal/config"
	"task-queue-service/internal/handler"
	"task-queue-service/internal/repository"
)

func main() {
	if err := run(); err != nil {
		slog.Error("API stopped", "error", err)
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

	routes := handler.New(db)
	server := &http.Server{
		Addr: ":" + cfg.HTTPPort,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
			defer cancel()
			routes.ServeHTTP(w, r.WithContext(ctx))
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       time.Minute,
	}
	listening := make(chan error, 1)
	go func() { listening <- server.ListenAndServe() }()
	slog.Info("starting API", "addr", server.Addr)

	select {
	case err := <-listening:
		return err
	case <-ctx.Done():
		stop()
		shutdown, cancel := context.WithTimeout(context.Background(), 17*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			server.Close()
			return err
		}
		if err := <-listening; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
