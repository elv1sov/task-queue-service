package tests

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"task-queue-service/internal/service"
)

func TestExecute(t *testing.T) {
	cases := []struct {
		kind, payload, want string
	}{
		{"uppercase", `{"text":"привет world"}`, `{"text":"ПРИВЕТ WORLD"}`},
		{"count_words", `{"text":"  один\tдва\nтри  "}`, `{"words":3}`},
		{"count_words", `{"text":""}`, `{"words":0}`},
		{"sleep", `{"seconds":0}`, `{"slept_seconds":0}`},
	}
	for _, tc := range cases {
		t.Run(tc.kind+"/"+tc.want, func(t *testing.T) {
			run, err := service.Prepare(tc.kind, json.RawMessage(tc.payload))
			if err != nil {
				t.Fatal(err)
			}
			got, err := run(context.Background())
			if err != nil || string(got) != tc.want {
				t.Fatalf("result = %s, error = %v, want %s", got, err, tc.want)
			}
		})
	}
}

func TestSleepCancellation(t *testing.T) {
	run, err := service.Prepare("sleep", json.RawMessage(`{"seconds":30}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := run(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline error, got %v", err)
	}
}

func TestInvalidPayload(t *testing.T) {
	cases := []struct{ kind, payload string }{
		{"unknown", `{}`}, {"uppercase", `null`}, {"uppercase", `{}`},
		{"uppercase", `{"text":null}`}, {"uppercase", `{"text":10}`},
		{"count_words", `{"text":"a","extra":3}`}, {"count_words", `{"text":"\u0000"}`},
		{"sleep", `{"seconds":31}`}, {"sleep", `{"seconds":-1}`},
		{"sleep", `{"seconds":0.5}`}, {"sleep", `{"seconds":null}`},
	}
	for _, tc := range cases {
		t.Run(tc.kind+"/"+tc.payload, func(t *testing.T) {
			if _, err := service.Prepare(tc.kind, json.RawMessage(tc.payload)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
