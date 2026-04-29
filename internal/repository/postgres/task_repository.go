package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		INSERT INTO tasks (title, description, status, recurrence_id, scheduled_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, title, description, status, recurrence_id, scheduled_date, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		task.Title, task.Description, task.Status,
		task.RecurrenceID, task.ScheduledDate,
		task.CreatedAt, task.UpdatedAt,
	)
	created, err := scanTask(row)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *Repository) CreateBatch(ctx context.Context, tasks []*taskdomain.Task) ([]taskdomain.Task, error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	const query = `
		INSERT INTO tasks (title, description, status, recurrence_id, scheduled_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, title, description, status, recurrence_id, scheduled_date, created_at, updated_at
	`

	batch := &pgx.Batch{}
	for _, t := range tasks {
		batch.Queue(query,
			t.Title, t.Description, t.Status,
			t.RecurrenceID, t.ScheduledDate,
			t.CreatedAt, t.UpdatedAt,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	created := make([]taskdomain.Task, 0, len(tasks))
	for range tasks {
		task, err := scanTask(results.QueryRow())
		if err != nil {
			return nil, fmt.Errorf("scan batch row: %w", err)
		}
		created = append(created, *task)
	}
	return created, nil
}

func (r *Repository) ListByRecurrenceID(ctx context.Context, recurrenceID int64) ([]taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, recurrence_id, scheduled_date, created_at, updated_at
		FROM tasks
		WHERE recurrence_id = $1
		ORDER BY scheduled_date ASC
	`

	rows, err := r.pool.Query(ctx, query, recurrenceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}
	return tasks, rows.Err()
}

func (r *Repository) DeleteFutureByRecurrenceID(ctx context.Context, recurrenceID int64, from time.Time) error {
	const query = `DELETE FROM tasks WHERE recurrence_id = $1 AND scheduled_date >= $2`
	_, err := r.pool.Exec(ctx, query, recurrenceID, from)
	return err
}

func (r *Repository) DetachPastByRecurrenceID(ctx context.Context, recurrenceID int64, before time.Time) error {
	const query = `
		UPDATE tasks
		SET recurrence_id = NULL
		WHERE recurrence_id = $1 AND (scheduled_date IS NULL OR scheduled_date < $2)
	`
	_, err := r.pool.Exec(ctx, query, recurrenceID, before)
	return err
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, recurrence_id, scheduled_date, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return found, nil
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		UPDATE tasks
		SET title = $1,
			description = $2,
			status = $3,
			updated_at = $4
		WHERE id = $5
		RETURNING id, title, description, status, recurrence_id, scheduled_date, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query, task.Title, task.Description, task.Status, task.UpdatedAt, task.ID)
	updated, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return updated, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

func (r *Repository) List(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, recurrence_id, scheduled_date, created_at, updated_at
		FROM tasks
		ORDER BY id DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task   taskdomain.Task
		status string
	)

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&task.RecurrenceID,
		&task.ScheduledDate,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)

	return &task, nil
}
