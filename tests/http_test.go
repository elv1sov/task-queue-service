package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"task-queue-service/internal/handler"
	"task-queue-service/internal/model"
	"task-queue-service/internal/service"
)

type fakeTasks struct {
	createErr, getErr, listErr, healthErr error
	created                               bool
	limit                                 int
}

func (f *fakeTasks) Create(_ context.Context, kind string, payload json.RawMessage) (model.Task, error) {
	f.created = true
	return model.Task{ID: 1, Type: kind, Payload: payload, Status: model.Pending}, f.createErr
}

func (f *fakeTasks) Get(_ context.Context, id int64) (model.Task, error) {
	return model.Task{ID: id, Status: model.Completed, Result: json.RawMessage(`{"text":"HELLO"}`)}, f.getErr
}

func (f *fakeTasks) List(_ context.Context, limit int) ([]model.Task, error) {
	f.limit = limit
	return []model.Task{}, f.listErr
}

func (f *fakeTasks) Ping(context.Context) error { return f.healthErr }

func TestHTTP(t *testing.T) {
	const valid = `{"type":"uppercase","payload":{"text":"hello"}}`
	unavailable := errors.New("connection refused")
	cases := []struct {
		name, method, path, body string
		fake                     fakeTasks
		status                   int
		created                  bool
		limit                    int
		location                 string
	}{
		{name: "create", method: "POST", path: "/api/tasks", body: valid, status: 201, created: true, location: "/api/tasks/1"},
		{name: "invalid task", method: "POST", path: "/api/tasks", body: `{"type":"sleep","payload":{"seconds":31}}`, status: 400},
		{name: "invalid json", method: "POST", path: "/api/tasks", body: "{", status: 400},
		{name: "trailing json", method: "POST", path: "/api/tasks", body: valid + "{}", status: 400},
		{name: "large body", method: "POST", path: "/api/tasks", body: valid + strings.Repeat(" ", 65536), status: 413},
		{name: "create error", method: "POST", path: "/api/tasks", body: valid, fake: fakeTasks{createErr: unavailable}, status: 503, created: true},
		{name: "get", method: "GET", path: "/api/tasks/1", status: 200},
		{name: "bad id", method: "GET", path: "/api/tasks/foo", status: 400},
		{name: "missing", method: "GET", path: "/api/tasks/1", fake: fakeTasks{getErr: service.ErrNotFound}, status: 404},
		{name: "get error", method: "GET", path: "/api/tasks/1", fake: fakeTasks{getErr: unavailable}, status: 503},
		{name: "list", method: "GET", path: "/api/tasks?limit=5", status: 200, limit: 5},
		{name: "list default", method: "GET", path: "/api/tasks", status: 200, limit: 20},
		{name: "bad limit", method: "GET", path: "/api/tasks?limit=101", status: 400},
		{name: "list error", method: "GET", path: "/api/tasks", fake: fakeTasks{listErr: unavailable}, status: 503, limit: 20},
		{name: "health", method: "GET", path: "/health", status: 200},
		{name: "health error", method: "GET", path: "/health", fake: fakeTasks{healthErr: unavailable}, status: 503},
		{name: "method", method: "DELETE", path: "/api/tasks/1", status: 405},
		{name: "unknown route", method: "GET", path: "/other", status: 404},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.New(&tc.fake).ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if w.Code != tc.status || tc.fake.created != tc.created || tc.fake.limit != tc.limit {
				t.Fatalf("status=%d created=%t limit=%d body=%s", w.Code, tc.fake.created, tc.fake.limit, w.Body)
			}
			if w.Header().Get("Location") != tc.location || w.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("unexpected headers: %v", w.Header())
			}
			var body struct {
				Data  json.RawMessage
				Error *string
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if (body.Error != nil) != (tc.status >= 400) || (string(body.Data) == "null") != (tc.status >= 400) {
				t.Fatalf("unexpected envelope: %s", w.Body)
			}
			if strings.Contains(w.Body.String(), unavailable.Error()) {
				t.Fatal("response exposed dependency error")
			}
		})
	}
}
