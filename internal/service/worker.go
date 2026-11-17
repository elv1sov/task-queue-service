package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"task-queue-service/internal/model"
)

type WorkerStore interface {
	Claim(context.Context) (model.Task, error)
	Finish(context.Context, int64, json.RawMessage, *string) error
}

type Worker struct {
	Store WorkerStore
}

func (w Worker) Run(ctx context.Context, concurrency int) {
	var workers sync.WaitGroup
	for range concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			w.consume(ctx)
		}()
	}
	workers.Wait()
}

func (w Worker) consume(ctx context.Context) {
	for ctx.Err() == nil {
		// Give an in-flight claim time to return even if shutdown begins.
		workCtx := context.WithoutCancel(ctx)
		claimCtx, cancel := context.WithTimeout(workCtx, 5*time.Second)
		task, err := w.Store.Claim(claimCtx)
		cancel()
		if err == nil {
			w.handle(workCtx, task)
			continue
		}
		if !errors.Is(err, ErrNotFound) {
			slog.Error("claim task", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (w Worker) handle(ctx context.Context, task model.Task) {
	runCtx, cancel := context.WithTimeout(ctx, (MaxSleepSeconds+2)*time.Second)
	run, err := Prepare(task.Type, task.Payload)
	var result json.RawMessage
	var taskErr *string
	if err == nil {
		result, err = run(runCtx)
	}
	cancel()
	if err != nil {
		message := err.Error()
		taskErr = &message
		slog.Error("execute task", "id", task.ID, "error", err)
	}

	saveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := w.Store.Finish(saveCtx, task.ID, result, taskErr); err != nil {
		slog.Error("save task result", "id", task.ID, "error", err)
	}
}
