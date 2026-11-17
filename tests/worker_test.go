package tests

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"task-queue-service/internal/model"
	"task-queue-service/internal/service"
)

type workerStore struct {
	cancel   context.CancelFunc
	task     model.Task
	claimErr error
	finished bool
	result   json.RawMessage
	taskErr  *string
}

func (s *workerStore) Claim(ctx context.Context) (model.Task, error) {
	if err := ctx.Err(); err != nil {
		return model.Task{}, err
	}
	s.cancel()
	return s.task, s.claimErr
}

func (s *workerStore) Finish(_ context.Context, _ int64, result json.RawMessage, taskErr *string) error {
	s.finished, s.result, s.taskErr = true, result, taskErr
	return nil
}

func TestWorkerFinishesClaimedTaskDuringShutdown(t *testing.T) {
	cases := []struct {
		name, payload  string
		claimErr       error
		finish, failed bool
	}{
		{name: "success", payload: `{"text":"hello"}`, finish: true},
		{name: "execution error", payload: `{}`, finish: true, failed: true},
		{name: "empty queue", claimErr: service.ErrNotFound},
		{name: "database error", claimErr: errors.New("connection refused")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &workerStore{
				task:     model.Task{ID: 1, Type: "uppercase", Payload: json.RawMessage(tc.payload)},
				claimErr: tc.claimErr,
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			store.cancel = cancel
			service.Worker{Store: store}.Run(ctx, 1)
			if store.finished != tc.finish || (store.taskErr != nil) != tc.failed {
				t.Fatalf("finished=%t, error=%v", store.finished, store.taskErr)
			}
			if tc.finish && !tc.failed && string(store.result) != `{"text":"HELLO"}` {
				t.Fatalf("unexpected result: %s", store.result)
			}
		})
	}
}
