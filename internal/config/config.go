package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	HTTPPort          string
	DatabaseURL       string
	WorkerConcurrency int
}

func Load() (Config, error) {
	cfg := Config{
		HTTPPort:    value("HTTP_PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if _, err := number("HTTP_PORT", cfg.HTTPPort, 65535); err != nil {
		return Config{}, err
	}
	concurrency, err := number("WORKER_CONCURRENCY", value("WORKER_CONCURRENCY", "4"), 128)
	cfg.WorkerConcurrency = concurrency
	return cfg, err
}

func value(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func number(name, value string, maxValue int) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > maxValue {
		return 0, fmt.Errorf("%s must be an integer between 1 and %d", name, maxValue)
	}
	return n, nil
}
