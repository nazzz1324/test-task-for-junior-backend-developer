package handlers

import (
	"time"

	domain "example.com/taskservice/internal/domain/recurrence"
	taskdomain "example.com/taskservice/internal/domain/task"
	recurrenceusecase "example.com/taskservice/internal/usecase/recurrence"
)

// ── request DTOs ─────────────────────────────────────────────────────────────

type createRecurrenceRequest struct {
	TaskTitle       string              `json:"task_title"`
	TaskDescription string              `json:"task_description"`
	Type            domain.Type         `json:"type"`
	IntervalDays    *int                `json:"interval_days,omitempty"`
	DayOfMonth      *int                `json:"day_of_month,omitempty"`
	SpecificDates   []time.Time         `json:"specific_dates,omitempty"`
	StartsAt        time.Time           `json:"starts_at"`
	EndsAt          *time.Time          `json:"ends_at,omitempty"`
}

type updateRecurrenceRequest struct {
	TaskTitle       string      `json:"task_title"`
	TaskDescription string      `json:"task_description"`
	IntervalDays    *int        `json:"interval_days,omitempty"`
	DayOfMonth      *int        `json:"day_of_month,omitempty"`
	SpecificDates   []time.Time `json:"specific_dates,omitempty"`
	EndsAt          *time.Time  `json:"ends_at,omitempty"`
}

// ── response DTOs ─────────────────────────────────────────────────────────────

type ruleDTO struct {
	ID              int64       `json:"id"`
	TaskTitle       string      `json:"task_title"`
	TaskDescription string      `json:"task_description"`
	Type            domain.Type `json:"type"`
	IntervalDays    *int        `json:"interval_days,omitempty"`
	DayOfMonth      *int        `json:"day_of_month,omitempty"`
	SpecificDates   []time.Time `json:"specific_dates,omitempty"`
	StartsAt        time.Time   `json:"starts_at"`
	EndsAt          *time.Time  `json:"ends_at,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
}

type createRecurrenceResponse struct {
	Rule      ruleDTO   `json:"rule"`
	Instances []taskDTO `json:"instances"`
}

type updateRecurrenceResponse struct {
	Rule      ruleDTO   `json:"rule"`
	Instances []taskDTO `json:"instances"`
}

// ── converters ────────────────────────────────────────────────────────────────

func newRuleDTO(r domain.Rule) ruleDTO {
	return ruleDTO{
		ID:              r.ID,
		TaskTitle:       r.TaskTitle,
		TaskDescription: r.TaskDescription,
		Type:            r.Type,
		IntervalDays:    r.IntervalDays,
		DayOfMonth:      r.DayOfMonth,
		SpecificDates:   r.SpecificDates,
		StartsAt:        r.StartsAt,
		EndsAt:          r.EndsAt,
		CreatedAt:       r.CreatedAt,
	}
}

func toTaskDTOs(tasks []taskdomain.Task) []taskDTO {
	dtos := make([]taskDTO, len(tasks))
	for i := range tasks {
		dtos[i] = newTaskDTO(&tasks[i])
	}
	return dtos
}

func (req createRecurrenceRequest) toCreateInput() recurrenceusecase.CreateInput {
	return recurrenceusecase.CreateInput{
		TaskTitle:       req.TaskTitle,
		TaskDescription: req.TaskDescription,
		Rule: recurrenceusecase.RuleInput{
			Type:          req.Type,
			IntervalDays:  req.IntervalDays,
			DayOfMonth:    req.DayOfMonth,
			SpecificDates: req.SpecificDates,
			StartsAt:      req.StartsAt,
			EndsAt:        req.EndsAt,
		},
	}
}

func (req updateRecurrenceRequest) toUpdateInput() recurrenceusecase.UpdateInput {
	return recurrenceusecase.UpdateInput{
		TaskTitle:       req.TaskTitle,
		TaskDescription: req.TaskDescription,
		IntervalDays:    req.IntervalDays,
		DayOfMonth:      req.DayOfMonth,
		SpecificDates:   req.SpecificDates,
		EndsAt:          req.EndsAt,
	}
}
