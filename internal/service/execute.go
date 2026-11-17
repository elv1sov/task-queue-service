package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

const MaxSleepSeconds = 30

// DecodeObject rejects unknown fields and additional JSON values.
func DecodeObject(reader io.Reader, dst any) error {
	decoder := json.NewDecoder(reader)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	if len(raw) == 0 || raw[0] != '{' {
		return errors.New("expected a JSON object")
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		if err != nil {
			return err
		}
		return errors.New("expected a single JSON object")
	}

	fields := json.NewDecoder(bytes.NewReader(raw))
	fields.DisallowUnknownFields()
	return fields.Decode(dst)
}

// Prepare validates a task without executing it.
func Prepare(kind string, payload json.RawMessage) (func(context.Context) (json.RawMessage, error), error) {
	switch kind {
	case "uppercase", "count_words":
		var input struct {
			Text *string `json:"text"`
		}
		if DecodeObject(bytes.NewReader(payload), &input) != nil || input.Text == nil ||
			strings.ContainsRune(*input.Text, 0) {
			return nil, errors.New("payload.text must be a string without NUL characters")
		}
		return func(context.Context) (json.RawMessage, error) {
			if kind == "uppercase" {
				return json.Marshal(map[string]string{"text": strings.ToUpper(*input.Text)})
			}
			return json.Marshal(map[string]int{"words": len(strings.Fields(*input.Text))})
		}, nil
	case "sleep":
		var input struct {
			Seconds *int `json:"seconds"`
		}
		if DecodeObject(bytes.NewReader(payload), &input) != nil || input.Seconds == nil ||
			*input.Seconds < 0 || *input.Seconds > MaxSleepSeconds {
			return nil, errors.New("payload.seconds must be an integer between 0 and 30")
		}
		return func(ctx context.Context) (json.RawMessage, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			timer := time.NewTimer(time.Duration(*input.Seconds) * time.Second)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-timer.C:
				return json.Marshal(map[string]int{"slept_seconds": *input.Seconds})
			}
		}, nil
	default:
		return nil, errors.New("unsupported task type")
	}
}
