package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"task-queue-service/internal/model"
	"task-queue-service/internal/service"
)

type Tasks interface {
	Create(context.Context, string, json.RawMessage) (model.Task, error)
	Get(context.Context, int64) (model.Task, error)
	List(context.Context, int) ([]model.Task, error)
	Ping(context.Context) error
}

type Handler struct {
	tasks Tasks
}

type response struct {
	Data  any     `json:"data"`
	Error *string `json:"error"`
}

func New(tasks Tasks) http.Handler {
	h := Handler{tasks: tasks}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tasks", h.collection)
	mux.HandleFunc("/api/tasks/{id}", h.get)
	mux.HandleFunc("/health", h.health)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fail(w, http.StatusNotFound, "route not found")
	})
	return mux
}

func (h Handler) collection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.create(w, r)
	case http.MethodGet:
		h.list(w, r)
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

func (h Handler) create(w http.ResponseWriter, r *http.Request) {
	body := http.MaxBytesReader(w, r.Body, 64<<10)
	defer body.Close()

	var input struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := service.DecodeObject(body, &input); err != nil {
		var sizeError *http.MaxBytesError
		if errors.As(err, &sizeError) {
			fail(w, http.StatusRequestEntityTooLarge, "body exceeds 64 KiB")
			return
		}
		fail(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if _, err := service.Prepare(input.Type, input.Payload); err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}

	task, err := h.tasks.Create(r.Context(), input.Type, input.Payload)
	if err != nil {
		unavailable(w, err)
		return
	}
	w.Header().Set("Location", taskURL(task.ID))
	write(w, http.StatusCreated, response{Data: task})
}

func (h Handler) get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		fail(w, http.StatusBadRequest, "id must be a positive integer")
		return
	}
	task, err := h.tasks.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			fail(w, http.StatusNotFound, "task not found")
		} else {
			unavailable(w, err)
		}
		return
	}
	write(w, http.StatusOK, response{Data: task})
}

func (h Handler) list(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if raw := r.URL.Query().Get("limit"); r.URL.Query().Has("limit") {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 100 {
			fail(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return
		}
		limit = n
	}
	tasks, err := h.tasks.List(r.Context(), limit)
	if err != nil {
		unavailable(w, err)
		return
	}
	write(w, http.StatusOK, response{Data: tasks})
}

func (h Handler) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.tasks.Ping(ctx); err != nil {
		unavailable(w, err)
		return
	}
	write(w, http.StatusOK, response{Data: map[string]string{"status": "ok"}})
}

func taskURL(id int64) string {
	return "/api/tasks/" + strconv.FormatInt(id, 10)
}

func unavailable(w http.ResponseWriter, err error) {
	slog.Error("dependency request failed", "error", err)
	fail(w, http.StatusServiceUnavailable, "dependency unavailable")
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	fail(w, http.StatusMethodNotAllowed, "method not allowed")
}

func fail(w http.ResponseWriter, status int, message string) {
	write(w, status, response{Error: &message})
}

func write(w http.ResponseWriter, status int, body response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Warn("write response", "error", err)
	}
}
