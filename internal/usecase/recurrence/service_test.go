package recurrence

// Internal package tests — access to unexported `now` field for deterministic time.

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "example.com/taskservice/internal/domain/recurrence"
	taskdomain "example.com/taskservice/internal/domain/task"
)

// ── mock repositories ─────────────────────────────────────────────────────────

type mockRecurrenceRepo struct {
	createFn  func(ctx context.Context, rule *domain.Rule) (*domain.Rule, error)
	getByIDFn func(ctx context.Context, id int64) (*domain.Rule, error)
	updateFn  func(ctx context.Context, rule *domain.Rule) (*domain.Rule, error)
	deleteFn  func(ctx context.Context, id int64) error
	listFn    func(ctx context.Context) ([]domain.Rule, error)
}

func (m *mockRecurrenceRepo) Create(ctx context.Context, rule *domain.Rule) (*domain.Rule, error) {
	return m.createFn(ctx, rule)
}
func (m *mockRecurrenceRepo) GetByID(ctx context.Context, id int64) (*domain.Rule, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockRecurrenceRepo) Update(ctx context.Context, rule *domain.Rule) (*domain.Rule, error) {
	return m.updateFn(ctx, rule)
}
func (m *mockRecurrenceRepo) Delete(ctx context.Context, id int64) error {
	return m.deleteFn(ctx, id)
}
func (m *mockRecurrenceRepo) List(ctx context.Context) ([]domain.Rule, error) {
	return m.listFn(ctx)
}

type mockTaskRepo struct {
	createBatchFn    func(ctx context.Context, tasks []*taskdomain.Task) ([]taskdomain.Task, error)
	listByRecFn      func(ctx context.Context, recurrenceID int64) ([]taskdomain.Task, error)
	deleteFutureFn   func(ctx context.Context, recurrenceID int64, from time.Time) error
	detachPastFn     func(ctx context.Context, recurrenceID int64, before time.Time) error
}

func (m *mockTaskRepo) CreateBatch(ctx context.Context, tasks []*taskdomain.Task) ([]taskdomain.Task, error) {
	return m.createBatchFn(ctx, tasks)
}
func (m *mockTaskRepo) ListByRecurrenceID(ctx context.Context, recurrenceID int64) ([]taskdomain.Task, error) {
	return m.listByRecFn(ctx, recurrenceID)
}
func (m *mockTaskRepo) DeleteFutureByRecurrenceID(ctx context.Context, recurrenceID int64, from time.Time) error {
	return m.deleteFutureFn(ctx, recurrenceID, from)
}
func (m *mockTaskRepo) DetachPastByRecurrenceID(ctx context.Context, recurrenceID int64, before time.Time) error {
	return m.detachPastFn(ctx, recurrenceID, before)
}

// ── helpers ───────────────────────────────────────────────────────────────────

var fixedNow = time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

func fixedClock() time.Time { return fixedNow }

func newTestService(repo Repository, taskRepo TaskRepository) *Service {
	s := NewService(repo, taskRepo)
	s.now = fixedClock
	return s
}

func intP(n int) *int { return &n }

// ── Create tests ──────────────────────────────────────────────────────────────

func TestService_Create_ValidInput(t *testing.T) {
	var capturedTasks []*taskdomain.Task

	rrepo := &mockRecurrenceRepo{
		createFn: func(_ context.Context, rule *domain.Rule) (*domain.Rule, error) {
			rule.ID = 1
			return rule, nil
		},
	}
	trepo := &mockTaskRepo{
		createBatchFn: func(_ context.Context, tasks []*taskdomain.Task) ([]taskdomain.Task, error) {
			capturedTasks = tasks
			out := make([]taskdomain.Task, len(tasks))
			for i, t := range tasks {
				out[i] = *t
				out[i].ID = int64(i + 1)
			}
			return out, nil
		},
	}

	svc := newTestService(rrepo, trepo)

	input := CreateInput{
		TaskTitle: "Standup",
		Rule: RuleInput{
			Type:         domain.TypeEveryNDays,
			IntervalDays: intP(1),
			StartsAt:     time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
			EndsAt:       func() *time.Time { t := time.Date(2024, 6, 20, 0, 0, 0, 0, time.UTC); return &t }(),
		},
	}

	result, err := svc.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Rule.ID != 1 {
		t.Errorf("rule ID: want 1, got %d", result.Rule.ID)
	}
	// Jun15..Jun20 daily = 6 instances
	if len(capturedTasks) != 6 {
		t.Errorf("want 6 task instances, got %d", len(capturedTasks))
	}
	// All tasks must carry the recurrence ID
	for _, task := range capturedTasks {
		if task.RecurrenceID == nil || *task.RecurrenceID != 1 {
			t.Errorf("task.RecurrenceID: want 1, got %v", task.RecurrenceID)
		}
	}
}

func TestService_Create_EmptyTitle(t *testing.T) {
	svc := newTestService(&mockRecurrenceRepo{}, &mockTaskRepo{})

	_, err := svc.Create(context.Background(), CreateInput{
		TaskTitle: "",
		Rule:      RuleInput{Type: domain.TypeEveryNDays, IntervalDays: intP(1), StartsAt: fixedNow},
	})

	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("want ErrInvalidInput, got %v", err)
	}
}

func TestService_Create_InvalidType(t *testing.T) {
	svc := newTestService(&mockRecurrenceRepo{}, &mockTaskRepo{})

	_, err := svc.Create(context.Background(), CreateInput{
		TaskTitle: "title",
		Rule:      RuleInput{Type: "bad_type", StartsAt: fixedNow},
	})

	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("want ErrInvalidInput, got %v", err)
	}
}

func TestService_Create_EveryNDays_MissingInterval(t *testing.T) {
	svc := newTestService(&mockRecurrenceRepo{}, &mockTaskRepo{})

	_, err := svc.Create(context.Background(), CreateInput{
		TaskTitle: "title",
		Rule: RuleInput{
			Type:     domain.TypeEveryNDays,
			StartsAt: fixedNow,
			// IntervalDays intentionally nil
		},
	})

	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("want ErrInvalidInput, got %v", err)
	}
}

func TestService_Create_MonthlyOnDay_OutOfRange(t *testing.T) {
	tests := []struct {
		name string
		day  int
	}{
		{"zero", 0},
		{"above 30", 31},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(&mockRecurrenceRepo{}, &mockTaskRepo{})
			_, err := svc.Create(context.Background(), CreateInput{
				TaskTitle: "title",
				Rule:      RuleInput{Type: domain.TypeMonthlyOnDay, DayOfMonth: intP(tt.day), StartsAt: fixedNow},
			})
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("day=%d: want ErrInvalidInput, got %v", tt.day, err)
			}
		})
	}
}

func TestService_Create_EndsAtBeforeStartsAt(t *testing.T) {
	svc := newTestService(&mockRecurrenceRepo{}, &mockTaskRepo{})

	endsAt := fixedNow.AddDate(0, 0, -1)
	_, err := svc.Create(context.Background(), CreateInput{
		TaskTitle: "title",
		Rule: RuleInput{
			Type: domain.TypeEveryNDays, IntervalDays: intP(1),
			StartsAt: fixedNow, EndsAt: &endsAt,
		},
	})

	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("want ErrInvalidInput, got %v", err)
	}
}

// ── Update tests ──────────────────────────────────────────────────────────────

func TestService_Update_DeletesFutureAndRegenerates(t *testing.T) {
	existingRule := &domain.Rule{
		ID: 5, TaskTitle: "Old title", Type: domain.TypeEveryNDays,
		IntervalDays: intP(1),
		StartsAt:     time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	var deletedFrom time.Time
	var createdBatch []*taskdomain.Task

	rrepo := &mockRecurrenceRepo{
		getByIDFn: func(_ context.Context, id int64) (*domain.Rule, error) {
			return existingRule, nil
		},
		updateFn: func(_ context.Context, rule *domain.Rule) (*domain.Rule, error) {
			return rule, nil
		},
	}
	trepo := &mockTaskRepo{
		deleteFutureFn: func(_ context.Context, _ int64, from time.Time) error {
			deletedFrom = from
			return nil
		},
		createBatchFn: func(_ context.Context, tasks []*taskdomain.Task) ([]taskdomain.Task, error) {
			createdBatch = tasks
			out := make([]taskdomain.Task, len(tasks))
			for i, t := range tasks {
				out[i] = *t
			}
			return out, nil
		},
	}

	svc := newTestService(rrepo, trepo)

	_, err := svc.Update(context.Background(), 5, UpdateInput{
		TaskTitle:    "New title",
		IntervalDays: intP(2),
		EndsAt: func() *time.Time {
			t := time.Date(2024, 6, 25, 0, 0, 0, 0, time.UTC)
			return &t
		}(),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// tomorrow = Jun16 (fixedNow = Jun15)
	wantFrom := time.Date(2024, 6, 16, 0, 0, 0, 0, time.UTC)
	if !deletedFrom.Equal(wantFrom) {
		t.Errorf("DeleteFuture from: want %s, got %s", wantFrom, deletedFrom)
	}
	if len(createdBatch) == 0 {
		t.Error("expected new instances to be created after update")
	}
}

func TestService_Update_NotFound(t *testing.T) {
	rrepo := &mockRecurrenceRepo{
		getByIDFn: func(_ context.Context, _ int64) (*domain.Rule, error) {
			return nil, domain.ErrNotFound
		},
	}
	svc := newTestService(rrepo, &mockTaskRepo{})

	_, err := svc.Update(context.Background(), 99, UpdateInput{TaskTitle: "t"})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

// ── Delete tests ──────────────────────────────────────────────────────────────

func TestService_Delete_DetachesPastThenDeletesRule(t *testing.T) {
	var detachCalled, deleteCalled bool
	var detachBefore time.Time

	rrepo := &mockRecurrenceRepo{
		deleteFn: func(_ context.Context, id int64) error {
			deleteCalled = true
			return nil
		},
	}
	trepo := &mockTaskRepo{
		detachPastFn: func(_ context.Context, _ int64, before time.Time) error {
			detachCalled = true
			detachBefore = before
			return nil
		},
	}

	svc := newTestService(rrepo, trepo)

	err := svc.Delete(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !detachCalled {
		t.Error("DetachPast was not called")
	}
	if !deleteCalled {
		t.Error("repo.Delete was not called")
	}
	// "today" with fixedNow = Jun15
	wantBefore := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	if !detachBefore.Equal(wantBefore) {
		t.Errorf("detach before: want %s, got %s", wantBefore, detachBefore)
	}
}

func TestService_Delete_InvalidID(t *testing.T) {
	svc := newTestService(&mockRecurrenceRepo{}, &mockTaskRepo{})
	if err := svc.Delete(context.Background(), 0); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("want ErrInvalidInput, got %v", err)
	}
}

// ── ExtendHorizon tests ───────────────────────────────────────────────────────

func TestService_ExtendHorizon_NoExistingInstances(t *testing.T) {
	rule := domain.Rule{
		ID: 1, TaskTitle: "Daily", Type: domain.TypeEveryNDays,
		IntervalDays: intP(1),
		StartsAt:     time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
	}

	var createdBatch []*taskdomain.Task

	rrepo := &mockRecurrenceRepo{
		listFn: func(_ context.Context) ([]domain.Rule, error) {
			return []domain.Rule{rule}, nil
		},
	}
	trepo := &mockTaskRepo{
		listByRecFn: func(_ context.Context, _ int64) ([]taskdomain.Task, error) {
			return nil, nil // no instances yet
		},
		createBatchFn: func(_ context.Context, tasks []*taskdomain.Task) ([]taskdomain.Task, error) {
			createdBatch = tasks
			out := make([]taskdomain.Task, len(tasks))
			for i, t := range tasks {
				out[i] = *t
			}
			return out, nil
		},
	}

	svc := newTestService(rrepo, trepo)

	until := time.Date(2024, 6, 20, 0, 0, 0, 0, time.UTC)
	if err := svc.ExtendHorizon(context.Background(), until); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Jun15..Jun20 = 6 instances
	if len(createdBatch) != 6 {
		t.Errorf("want 6 instances, got %d", len(createdBatch))
	}
}

func TestService_ExtendHorizon_WithExistingInstances_GeneratesFromNextDay(t *testing.T) {
	latestDate := time.Date(2024, 6, 17, 0, 0, 0, 0, time.UTC)
	rule := domain.Rule{
		ID: 2, TaskTitle: "Daily", Type: domain.TypeEveryNDays,
		IntervalDays: intP(1),
		StartsAt:     time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
	}

	var batchDates []time.Time

	rrepo := &mockRecurrenceRepo{
		listFn: func(_ context.Context) ([]domain.Rule, error) {
			return []domain.Rule{rule}, nil
		},
	}
	trepo := &mockTaskRepo{
		listByRecFn: func(_ context.Context, _ int64) ([]taskdomain.Task, error) {
			return []taskdomain.Task{
				{ScheduledDate: func() *time.Time { t := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC); return &t }()},
				{ScheduledDate: &latestDate},
			}, nil
		},
		createBatchFn: func(_ context.Context, tasks []*taskdomain.Task) ([]taskdomain.Task, error) {
			for _, t := range tasks {
				batchDates = append(batchDates, *t.ScheduledDate)
			}
			out := make([]taskdomain.Task, len(tasks))
			for i, t := range tasks {
				out[i] = *t
			}
			return out, nil
		},
	}

	svc := newTestService(rrepo, trepo)

	until := time.Date(2024, 6, 20, 0, 0, 0, 0, time.UTC)
	if err := svc.ExtendHorizon(context.Background(), until); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Existing: Jun15, Jun17. Next: Jun18, Jun19, Jun20.
	want := []time.Time{
		time.Date(2024, 6, 18, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 6, 19, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 6, 20, 0, 0, 0, 0, time.UTC),
	}
	if len(batchDates) != len(want) {
		t.Fatalf("want %d new instances, got %d", len(want), len(batchDates))
	}
	for i := range want {
		if !batchDates[i].Equal(want[i]) {
			t.Errorf("date[%d]: want %s, got %s", i, want[i], batchDates[i])
		}
	}
}

func TestService_ExtendHorizon_AlreadyAtHorizon_DoesNothing(t *testing.T) {
	latestDate := time.Date(2024, 6, 20, 0, 0, 0, 0, time.UTC)
	rule := domain.Rule{
		ID: 3, TaskTitle: "Daily", Type: domain.TypeEveryNDays,
		IntervalDays: intP(1),
		StartsAt:     time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
	}

	createCalled := false
	rrepo := &mockRecurrenceRepo{
		listFn: func(_ context.Context) ([]domain.Rule, error) {
			return []domain.Rule{rule}, nil
		},
	}
	trepo := &mockTaskRepo{
		listByRecFn: func(_ context.Context, _ int64) ([]taskdomain.Task, error) {
			return []taskdomain.Task{{ScheduledDate: &latestDate}}, nil
		},
		createBatchFn: func(_ context.Context, _ []*taskdomain.Task) ([]taskdomain.Task, error) {
			createCalled = true
			return nil, nil
		},
	}

	svc := newTestService(rrepo, trepo)

	until := time.Date(2024, 6, 20, 0, 0, 0, 0, time.UTC) // same as latest
	if err := svc.ExtendHorizon(context.Background(), until); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createCalled {
		t.Error("CreateBatch should not be called when horizon is already met")
	}
}
