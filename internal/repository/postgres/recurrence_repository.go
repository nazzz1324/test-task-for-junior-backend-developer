package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "example.com/taskservice/internal/domain/recurrence"
)

type RecurrenceRepository struct {
	pool *pgxpool.Pool
}

func NewRecurrenceRepository(pool *pgxpool.Pool) *RecurrenceRepository {
	return &RecurrenceRepository{pool: pool}
}

func (r *RecurrenceRepository) Create(ctx context.Context, rule *domain.Rule) (*domain.Rule, error) {
	const query = `
		INSERT INTO task_recurrences
			(task_title, task_description, type, interval_days, day_of_month, specific_dates, starts_at, ends_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, task_title, task_description, type, interval_days, day_of_month, specific_dates, starts_at, ends_at, created_at
	`

	row := r.pool.QueryRow(ctx, query,
		rule.TaskTitle,
		rule.TaskDescription,
		string(rule.Type),
		rule.IntervalDays,
		rule.DayOfMonth,
		timesToDates(rule.SpecificDates),
		rule.StartsAt,
		rule.EndsAt,
		time.Now().UTC(),
	)
	return scanRule(row)
}

func (r *RecurrenceRepository) GetByID(ctx context.Context, id int64) (*domain.Rule, error) {
	const query = `
		SELECT id, task_title, task_description, type, interval_days, day_of_month, specific_dates, starts_at, ends_at, created_at
		FROM task_recurrences
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	rule, err := scanRule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return rule, nil
}

func (r *RecurrenceRepository) Update(ctx context.Context, rule *domain.Rule) (*domain.Rule, error) {
	const query = `
		UPDATE task_recurrences
		SET task_title       = $1,
		    task_description = $2,
		    interval_days    = $3,
		    day_of_month     = $4,
		    specific_dates   = $5,
		    ends_at          = $6
		WHERE id = $7
		RETURNING id, task_title, task_description, type, interval_days, day_of_month, specific_dates, starts_at, ends_at, created_at
	`

	row := r.pool.QueryRow(ctx, query,
		rule.TaskTitle,
		rule.TaskDescription,
		rule.IntervalDays,
		rule.DayOfMonth,
		timesToDates(rule.SpecificDates),
		rule.EndsAt,
		rule.ID,
	)
	updated, err := scanRule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return updated, nil
}

func (r *RecurrenceRepository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM task_recurrences WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *RecurrenceRepository) List(ctx context.Context) ([]domain.Rule, error) {
	const query = `
		SELECT id, task_title, task_description, type, interval_days, day_of_month, specific_dates, starts_at, ends_at, created_at
		FROM task_recurrences
		ORDER BY id DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := make([]domain.Rule, 0)
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, *rule)
	}
	return rules, rows.Err()
}

// ── scan helpers ─────────────────────────────────────────────────────────────

type ruleScanner interface {
	Scan(dest ...any) error
}

func scanRule(s ruleScanner) (*domain.Rule, error) {
	var (
		rule        domain.Rule
		ruleType    string
		dateArr     pgtype.Array[pgtype.Date]
	)

	if err := s.Scan(
		&rule.ID,
		&rule.TaskTitle,
		&rule.TaskDescription,
		&ruleType,
		&rule.IntervalDays,
		&rule.DayOfMonth,
		&dateArr,
		&rule.StartsAt,
		&rule.EndsAt,
		&rule.CreatedAt,
	); err != nil {
		return nil, err
	}

	rule.Type = domain.Type(ruleType)
	rule.SpecificDates = datesToTimes(dateArr)
	return &rule, nil
}

func timesToDates(ts []time.Time) pgtype.Array[pgtype.Date] {
	if len(ts) == 0 {
		return pgtype.Array[pgtype.Date]{Valid: false}
	}
	elems := make([]pgtype.Date, len(ts))
	for i, t := range ts {
		elems[i] = pgtype.Date{
			Time:  t.UTC(),
			Valid: true,
		}
	}
	return pgtype.Array[pgtype.Date]{Elements: elems, Dims: []pgtype.ArrayDimension{{Length: int32(len(elems)), LowerBound: 1}}, Valid: true}
}

func datesToTimes(arr pgtype.Array[pgtype.Date]) []time.Time {
	if !arr.Valid || len(arr.Elements) == 0 {
		return nil
	}
	ts := make([]time.Time, 0, len(arr.Elements))
	for _, d := range arr.Elements {
		if d.Valid {
			ts = append(ts, d.Time.UTC())
		}
	}
	return ts
}
