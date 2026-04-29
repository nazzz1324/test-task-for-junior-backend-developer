package recurrence

import (
	"context"
	"time"

	domain "example.com/taskservice/internal/domain/recurrence"
	taskdomain "example.com/taskservice/internal/domain/task"
)

// Repository manages persistence of recurrence rules.
type Repository interface {
	Create(ctx context.Context, rule *domain.Rule) (*domain.Rule, error)
	GetByID(ctx context.Context, id int64) (*domain.Rule, error)
	Update(ctx context.Context, rule *domain.Rule) (*domain.Rule, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]domain.Rule, error)
}

// TaskRepository is the subset of task persistence operations used by the recurrence service.
type TaskRepository interface {
	CreateBatch(ctx context.Context, tasks []*taskdomain.Task) ([]taskdomain.Task, error)
	ListByRecurrenceID(ctx context.Context, recurrenceID int64) ([]taskdomain.Task, error)
	DeleteFutureByRecurrenceID(ctx context.Context, recurrenceID int64, from time.Time) error
	DetachPastByRecurrenceID(ctx context.Context, recurrenceID int64, before time.Time) error
}

// Usecase is the public contract for the recurrence feature.
type Usecase interface {
	Create(ctx context.Context, input CreateInput) (*CreateResult, error)
	GetByID(ctx context.Context, id int64) (*domain.Rule, error)
	Update(ctx context.Context, id int64, input UpdateInput) (*UpdateResult, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]domain.Rule, error)
	ListInstances(ctx context.Context, ruleID int64) ([]taskdomain.Task, error)
	ExtendHorizon(ctx context.Context, until time.Time) error
}

// CreateInput is the payload for creating a new recurring task series.
type CreateInput struct {
	TaskTitle       string
	TaskDescription string
	Rule            RuleInput
}

// RuleInput holds the recurrence parameters supplied by the caller.
type RuleInput struct {
	Type          domain.Type
	IntervalDays  *int
	DayOfMonth    *int
	SpecificDates []time.Time
	StartsAt      time.Time
	EndsAt        *time.Time
}

// UpdateInput is the mutable subset of a recurrence rule.
// Type and StartsAt are immutable after creation.
type UpdateInput struct {
	TaskTitle       string
	TaskDescription string
	IntervalDays    *int
	DayOfMonth      *int
	SpecificDates   []time.Time
	EndsAt          *time.Time
}

// CreateResult bundles the persisted rule with its initial task instances.
type CreateResult struct {
	Rule      domain.Rule
	Instances []taskdomain.Task
}

// UpdateResult bundles the updated rule with the regenerated future instances.
type UpdateResult struct {
	Rule      domain.Rule
	Instances []taskdomain.Task
}
