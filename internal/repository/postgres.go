package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"task-queue-service/internal/model"
	"task-queue-service/internal/service"
)

const columns = "id, type, payload, status, result, error, created_at, updated_at"

type Postgres struct {
	*pgxpool.Pool
}

func Open(ctx context.Context, url string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{Pool: pool}, nil
}

func scan(row pgx.Row) (model.Task, error) {
	var task model.Task
	err := row.Scan(&task.ID, &task.Type, &task.Payload, &task.Status, &task.Result,
		&task.Error, &task.CreatedAt, &task.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return task, service.ErrNotFound
	}
	return task, err
}

func (db *Postgres) Create(ctx context.Context, kind string, payload json.RawMessage) (model.Task, error) {
	return scan(db.QueryRow(ctx, "INSERT INTO tasks (type, payload) VALUES ($1, $2) RETURNING "+columns, kind, payload))
}

func (db *Postgres) Get(ctx context.Context, id int64) (model.Task, error) {
	return scan(db.QueryRow(ctx, "SELECT "+columns+" FROM tasks WHERE id = $1", id))
}

func (db *Postgres) List(ctx context.Context, limit int) ([]model.Task, error) {
	rows, err := db.Query(ctx, "SELECT "+columns+" FROM tasks ORDER BY id DESC LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]model.Task, 0)
	for rows.Next() {
		task, err := scan(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// Claim takes the oldest available pending task in one SQL statement.
func (db *Postgres) Claim(ctx context.Context) (model.Task, error) {
	return scan(db.QueryRow(ctx, `UPDATE tasks SET status = 'running', updated_at = now()
		WHERE id = (
			SELECT id FROM tasks WHERE status = 'pending'
			ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED
		)
		RETURNING `+columns))
}

func (db *Postgres) Finish(ctx context.Context, id int64, result json.RawMessage, taskErr *string) error {
	status := model.Completed
	if taskErr != nil {
		status = model.Failed
	}
	tag, err := db.Exec(ctx, `UPDATE tasks SET status = $2, result = $3, error = $4, updated_at = now()
		WHERE id = $1 AND status = 'running'`, id, status, result, taskErr)
	if err == nil && tag.RowsAffected() != 1 {
		return service.ErrNotFound
	}
	return err
}
