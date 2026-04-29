package recurrence

import (
	"context"
	"fmt"
	"strings"
	"time"

	domain "example.com/taskservice/internal/domain/recurrence"
	taskdomain "example.com/taskservice/internal/domain/task"
)

const (
	defaultHorizonDays = 90
	maxInstances       = 365
)

type Service struct {
	repo     Repository
	taskRepo TaskRepository
	now      func() time.Time
}

func NewService(repo Repository, taskRepo TaskRepository) *Service {
	return &Service{
		repo:     repo,
		taskRepo: taskRepo,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*CreateResult, error) {
	if err := validateCreateInput(input); err != nil {
		return nil, err
	}

	rule := buildRule(input)
	created, err := s.repo.Create(ctx, &rule)
	if err != nil {
		return nil, fmt.Errorf("create recurrence rule: %w", err)
	}

	dates := s.futureDates(created, created.StartsAt)
	instances, err := s.createInstances(ctx, created, dates)
	if err != nil {
		return nil, err
	}

	return &CreateResult{Rule: *created, Instances: instances}, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*domain.Rule, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}
	rule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get recurrence rule: %w", err)
	}
	return rule, nil
}

// Update applies mutable field changes to a rule, then deletes all future
// instances and regenerates them based on the updated rule.
func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*UpdateResult, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get recurrence rule: %w", err)
	}

	applyUpdate(existing, input)
	if err := validateAfterUpdate(existing, input); err != nil {
		return nil, err
	}

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return nil, fmt.Errorf("update recurrence rule: %w", err)
	}

	// Delete everything from tomorrow onward, then regenerate.
	tomorrow := dayUTC(s.now()).AddDate(0, 0, 1)
	if err := s.taskRepo.DeleteFutureByRecurrenceID(ctx, id, tomorrow); err != nil {
		return nil, fmt.Errorf("delete future instances: %w", err)
	}

	dates := s.futureDates(updated, tomorrow)
	instances, err := s.createInstances(ctx, updated, dates)
	if err != nil {
		return nil, err
	}

	return &UpdateResult{Rule: *updated, Instances: instances}, nil
}

// Delete detaches past task instances (preserving history), then removes the
// rule — which cascades to delete remaining future instances.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	today := dayUTC(s.now())
	if err := s.taskRepo.DetachPastByRecurrenceID(ctx, id, today); err != nil {
		return fmt.Errorf("detach past instances: %w", err)
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete recurrence rule: %w", err)
	}
	return nil
}

func (s *Service) List(ctx context.Context) ([]domain.Rule, error) {
	rules, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recurrence rules: %w", err)
	}
	return rules, nil
}

func (s *Service) ListInstances(ctx context.Context, ruleID int64) ([]taskdomain.Task, error) {
	if ruleID <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}
	instances, err := s.taskRepo.ListByRecurrenceID(ctx, ruleID)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}
	return instances, nil
}

// ExtendHorizon generates missing task instances for all active rules up to until.
// Called by the scheduler to maintain a rolling pre-generation window.
func (s *Service) ExtendHorizon(ctx context.Context, until time.Time) error {
	rules, err := s.repo.List(ctx)
	if err != nil {
		return fmt.Errorf("list rules for horizon extension: %w", err)
	}
	for i := range rules {
		if err := s.extendRule(ctx, &rules[i], until); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) extendRule(ctx context.Context, rule *domain.Rule, until time.Time) error {
	instances, err := s.taskRepo.ListByRecurrenceID(ctx, rule.ID)
	if err != nil {
		return fmt.Errorf("list instances for rule %d: %w", rule.ID, err)
	}

	// Start generating from the day after the latest existing instance.
	from := dayUTC(rule.StartsAt)
	for _, inst := range instances {
		if inst.ScheduledDate != nil {
			if d := dayUTC(*inst.ScheduledDate); d.After(from) {
				from = d
			}
		}
	}
	if len(instances) > 0 {
		from = from.AddDate(0, 0, 1)
	}

	if !from.Before(until) {
		return nil
	}

	dates := filterUntil(NextDates(*rule, from, maxInstances), until)
	if len(dates) == 0 {
		return nil
	}
	_, err = s.createInstances(ctx, rule, dates)
	return err
}

func (s *Service) futureDates(rule *domain.Rule, from time.Time) []time.Time {
	horizon := s.now().AddDate(0, 0, defaultHorizonDays)
	if rule.EndsAt != nil && rule.EndsAt.Before(horizon) {
		horizon = *rule.EndsAt
	}
	return filterUntil(NextDates(*rule, from, maxInstances), horizon)
}

func (s *Service) createInstances(ctx context.Context, rule *domain.Rule, dates []time.Time) ([]taskdomain.Task, error) {
	if len(dates) == 0 {
		return nil, nil
	}
	now := s.now()
	tasks := make([]*taskdomain.Task, 0, len(dates))
	for _, d := range dates {
		d := d
		tasks = append(tasks, &taskdomain.Task{
			Title:         rule.TaskTitle,
			Description:   rule.TaskDescription,
			Status:        taskdomain.StatusNew,
			RecurrenceID:  &rule.ID,
			ScheduledDate: &d,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}
	return s.taskRepo.CreateBatch(ctx, tasks)
}

// ── helpers ──────────────────────────────────────────────────────────────────

func dayUTC(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func filterUntil(dates []time.Time, until time.Time) []time.Time {
	until = dayUTC(until)
	out := make([]time.Time, 0, len(dates))
	for _, d := range dates {
		if !d.After(until) {
			out = append(out, d)
		}
	}
	return out
}

func buildRule(input CreateInput) domain.Rule {
	return domain.Rule{
		TaskTitle:       strings.TrimSpace(input.TaskTitle),
		TaskDescription: strings.TrimSpace(input.TaskDescription),
		Type:            input.Rule.Type,
		IntervalDays:    input.Rule.IntervalDays,
		DayOfMonth:      input.Rule.DayOfMonth,
		SpecificDates:   input.Rule.SpecificDates,
		StartsAt:        dayUTC(input.Rule.StartsAt),
		EndsAt:          input.Rule.EndsAt,
	}
}

func applyUpdate(rule *domain.Rule, input UpdateInput) {
	rule.TaskTitle = strings.TrimSpace(input.TaskTitle)
	rule.TaskDescription = strings.TrimSpace(input.TaskDescription)
	rule.IntervalDays = input.IntervalDays
	rule.DayOfMonth = input.DayOfMonth
	rule.SpecificDates = input.SpecificDates
	rule.EndsAt = input.EndsAt
}

func validateCreateInput(input CreateInput) error {
	if strings.TrimSpace(input.TaskTitle) == "" {
		return fmt.Errorf("%w: task title is required", ErrInvalidInput)
	}
	return validateRuleInput(input.Rule)
}

func validateAfterUpdate(rule *domain.Rule, input UpdateInput) error {
	if strings.TrimSpace(input.TaskTitle) == "" {
		return fmt.Errorf("%w: task title is required", ErrInvalidInput)
	}
	return validateTypeSpecific(rule.Type, input.IntervalDays, input.DayOfMonth, input.SpecificDates)
}

func validateRuleInput(r RuleInput) error {
	if !r.Type.Valid() {
		return fmt.Errorf("%w: invalid recurrence type %q", ErrInvalidInput, r.Type)
	}
	if r.StartsAt.IsZero() {
		return fmt.Errorf("%w: starts_at is required", ErrInvalidInput)
	}
	if r.EndsAt != nil && !r.EndsAt.After(r.StartsAt) {
		return fmt.Errorf("%w: ends_at must be after starts_at", ErrInvalidInput)
	}
	return validateTypeSpecific(r.Type, r.IntervalDays, r.DayOfMonth, r.SpecificDates)
}

func validateTypeSpecific(t domain.Type, intervalDays, dayOfMonth *int, specificDates []time.Time) error {
	switch t {
	case domain.TypeEveryNDays:
		if intervalDays == nil || *intervalDays < 1 {
			return fmt.Errorf("%w: interval_days must be >= 1 for type every_n_days", ErrInvalidInput)
		}
	case domain.TypeMonthlyOnDay:
		if dayOfMonth == nil || *dayOfMonth < 1 || *dayOfMonth > 30 {
			return fmt.Errorf("%w: day_of_month must be between 1 and 30 for type monthly_on_day", ErrInvalidInput)
		}
	case domain.TypeSpecificDates:
		if len(specificDates) == 0 {
			return fmt.Errorf("%w: specific_dates must not be empty for type specific_dates", ErrInvalidInput)
		}
	}
	return nil
}
