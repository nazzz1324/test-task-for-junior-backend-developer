package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	domain "example.com/taskservice/internal/domain/recurrence"
	taskdomain "example.com/taskservice/internal/domain/task"
	"example.com/taskservice/internal/transport/http/handlers"
	recurrenceusecase "example.com/taskservice/internal/usecase/recurrence"
)

// ── mock Usecase ──────────────────────────────────────────────────────────────

type mockRecurrenceUsecase struct {
	createFn        func(ctx context.Context, input recurrenceusecase.CreateInput) (*recurrenceusecase.CreateResult, error)
	getByIDFn       func(ctx context.Context, id int64) (*domain.Rule, error)
	updateFn        func(ctx context.Context, id int64, input recurrenceusecase.UpdateInput) (*recurrenceusecase.UpdateResult, error)
	deleteFn        func(ctx context.Context, id int64) error
	listFn          func(ctx context.Context) ([]domain.Rule, error)
	listInstancesFn func(ctx context.Context, ruleID int64) ([]taskdomain.Task, error)
	extendFn        func(ctx context.Context, until time.Time) error
}

func (m *mockRecurrenceUsecase) Create(ctx context.Context, input recurrenceusecase.CreateInput) (*recurrenceusecase.CreateResult, error) {
	return m.createFn(ctx, input)
}
func (m *mockRecurrenceUsecase) GetByID(ctx context.Context, id int64) (*domain.Rule, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockRecurrenceUsecase) Update(ctx context.Context, id int64, input recurrenceusecase.UpdateInput) (*recurrenceusecase.UpdateResult, error) {
	return m.updateFn(ctx, id, input)
}
func (m *mockRecurrenceUsecase) Delete(ctx context.Context, id int64) error {
	return m.deleteFn(ctx, id)
}
func (m *mockRecurrenceUsecase) List(ctx context.Context) ([]domain.Rule, error) {
	return m.listFn(ctx)
}
func (m *mockRecurrenceUsecase) ListInstances(ctx context.Context, ruleID int64) ([]taskdomain.Task, error) {
	return m.listInstancesFn(ctx, ruleID)
}
func (m *mockRecurrenceUsecase) ExtendHorizon(ctx context.Context, until time.Time) error {
	if m.extendFn != nil {
		return m.extendFn(ctx, until)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildRecurrenceRouter(uc recurrenceusecase.Usecase) *mux.Router {
	h := handlers.NewRecurrenceHandler(uc)
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/tasks/recurring", h.Create).Methods(http.MethodPost)
	api.HandleFunc("/tasks/recurring", h.List).Methods(http.MethodGet)
	api.HandleFunc("/tasks/recurring/{id:[0-9]+}", h.GetByID).Methods(http.MethodGet)
	api.HandleFunc("/tasks/recurring/{id:[0-9]+}", h.Update).Methods(http.MethodPut)
	api.HandleFunc("/tasks/recurring/{id:[0-9]+}", h.Delete).Methods(http.MethodDelete)
	api.HandleFunc("/tasks/recurring/{id:[0-9]+}/instances", h.ListInstances).Methods(http.MethodGet)
	return r
}

func do(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(dst); err != nil {
		t.Fatalf("decode response body: %v\nbody: %s", err, rr.Body.String())
	}
}

// ── POST /api/v1/tasks/recurring ──────────────────────────────────────────────

func TestRecurrenceHandler_Create_Success(t *testing.T) {
	ruleID := int64(1)
	uc := &mockRecurrenceUsecase{
		createFn: func(_ context.Context, input recurrenceusecase.CreateInput) (*recurrenceusecase.CreateResult, error) {
			return &recurrenceusecase.CreateResult{
				Rule: domain.Rule{
					ID: ruleID, TaskTitle: input.TaskTitle,
					Type: input.Rule.Type, IntervalDays: input.Rule.IntervalDays,
					StartsAt: input.Rule.StartsAt, CreatedAt: time.Now(),
				},
				Instances: []taskdomain.Task{
					{ID: 1, Title: input.TaskTitle, Status: taskdomain.StatusNew},
				},
			}, nil
		},
	}

	body := map[string]any{
		"task_title": "Daily standup",
		"type":       "every_n_days",
		"interval_days": 1,
		"starts_at":  "2024-06-15T00:00:00Z",
	}

	rr := do(t, buildRecurrenceRouter(uc), http.MethodPost, "/api/v1/tasks/recurring", body)

	if rr.Code != http.StatusCreated {
		t.Errorf("status: want 201, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Rule      map[string]any   `json:"rule"`
		Instances []map[string]any `json:"instances"`
	}
	decodeBody(t, rr, &resp)

	if resp.Rule["id"] == nil {
		t.Error("rule.id missing in response")
	}
	if len(resp.Instances) != 1 {
		t.Errorf("want 1 instance, got %d", len(resp.Instances))
	}
}

func TestRecurrenceHandler_Create_InvalidJSON(t *testing.T) {
	uc := &mockRecurrenceUsecase{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/recurring",
		strings.NewReader(`{bad json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	buildRecurrenceRouter(uc).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestRecurrenceHandler_Create_UsecaseValidationError(t *testing.T) {
	uc := &mockRecurrenceUsecase{
		createFn: func(_ context.Context, _ recurrenceusecase.CreateInput) (*recurrenceusecase.CreateResult, error) {
			return nil, recurrenceusecase.ErrInvalidInput
		},
	}
	body := map[string]any{
		"task_title": "t", "type": "every_n_days", "interval_days": 0,
		"starts_at": "2024-06-15T00:00:00Z",
	}
	rr := do(t, buildRecurrenceRouter(uc), http.MethodPost, "/api/v1/tasks/recurring", body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

// ── GET /api/v1/tasks/recurring ───────────────────────────────────────────────

func TestRecurrenceHandler_List(t *testing.T) {
	uc := &mockRecurrenceUsecase{
		listFn: func(_ context.Context) ([]domain.Rule, error) {
			return []domain.Rule{
				{ID: 1, TaskTitle: "Rule A", Type: domain.TypeEveryNDays, IntervalDays: func() *int { n := 1; return &n }()},
				{ID: 2, TaskTitle: "Rule B", Type: domain.TypeOddDays},
			}, nil
		},
	}

	rr := do(t, buildRecurrenceRouter(uc), http.MethodGet, "/api/v1/tasks/recurring", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}

	var rules []map[string]any
	decodeBody(t, rr, &rules)
	if len(rules) != 2 {
		t.Errorf("want 2 rules, got %d", len(rules))
	}
}

// ── GET /api/v1/tasks/recurring/{id} ─────────────────────────────────────────

func TestRecurrenceHandler_GetByID_Found(t *testing.T) {
	uc := &mockRecurrenceUsecase{
		getByIDFn: func(_ context.Context, id int64) (*domain.Rule, error) {
			return &domain.Rule{ID: id, TaskTitle: "found", Type: domain.TypeEvenDays}, nil
		},
	}

	rr := do(t, buildRecurrenceRouter(uc), http.MethodGet, "/api/v1/tasks/recurring/5", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var rule map[string]any
	decodeBody(t, rr, &rule)
	if rule["task_title"] != "found" {
		t.Errorf("task_title: want 'found', got %v", rule["task_title"])
	}
}

func TestRecurrenceHandler_GetByID_NotFound(t *testing.T) {
	uc := &mockRecurrenceUsecase{
		getByIDFn: func(_ context.Context, _ int64) (*domain.Rule, error) {
			return nil, domain.ErrNotFound
		},
	}
	rr := do(t, buildRecurrenceRouter(uc), http.MethodGet, "/api/v1/tasks/recurring/999", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

// ── PUT /api/v1/tasks/recurring/{id} ─────────────────────────────────────────

func TestRecurrenceHandler_Update_Success(t *testing.T) {
	n := 2
	uc := &mockRecurrenceUsecase{
		updateFn: func(_ context.Context, id int64, input recurrenceusecase.UpdateInput) (*recurrenceusecase.UpdateResult, error) {
			return &recurrenceusecase.UpdateResult{
				Rule: domain.Rule{
					ID: id, TaskTitle: input.TaskTitle,
					Type: domain.TypeEveryNDays, IntervalDays: input.IntervalDays,
				},
				Instances: nil,
			}, nil
		},
	}

	body := map[string]any{
		"task_title":    "Updated title",
		"interval_days": n,
	}
	rr := do(t, buildRecurrenceRouter(uc), http.MethodPut, "/api/v1/tasks/recurring/3", body)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	decodeBody(t, rr, &resp)
	rule, _ := resp["rule"].(map[string]any)
	if rule["task_title"] != "Updated title" {
		t.Errorf("task_title: want 'Updated title', got %v", rule["task_title"])
	}
}

func TestRecurrenceHandler_Update_NotFound(t *testing.T) {
	uc := &mockRecurrenceUsecase{
		updateFn: func(_ context.Context, _ int64, _ recurrenceusecase.UpdateInput) (*recurrenceusecase.UpdateResult, error) {
			return nil, domain.ErrNotFound
		},
	}
	rr := do(t, buildRecurrenceRouter(uc), http.MethodPut, "/api/v1/tasks/recurring/99",
		map[string]any{"task_title": "x"})
	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

// ── DELETE /api/v1/tasks/recurring/{id} ──────────────────────────────────────

func TestRecurrenceHandler_Delete_Success(t *testing.T) {
	uc := &mockRecurrenceUsecase{
		deleteFn: func(_ context.Context, _ int64) error { return nil },
	}
	rr := do(t, buildRecurrenceRouter(uc), http.MethodDelete, "/api/v1/tasks/recurring/1", nil)
	if rr.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", rr.Code)
	}
}

func TestRecurrenceHandler_Delete_NotFound(t *testing.T) {
	uc := &mockRecurrenceUsecase{
		deleteFn: func(_ context.Context, _ int64) error { return domain.ErrNotFound },
	}
	rr := do(t, buildRecurrenceRouter(uc), http.MethodDelete, "/api/v1/tasks/recurring/99", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

// ── GET /api/v1/tasks/recurring/{id}/instances ───────────────────────────────

func TestRecurrenceHandler_ListInstances(t *testing.T) {
	recID := int64(7)
	scheduled := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	uc := &mockRecurrenceUsecase{
		listInstancesFn: func(_ context.Context, ruleID int64) ([]taskdomain.Task, error) {
			return []taskdomain.Task{
				{ID: 10, Title: "t", Status: taskdomain.StatusNew, RecurrenceID: &recID, ScheduledDate: &scheduled},
			}, nil
		},
	}

	rr := do(t, buildRecurrenceRouter(uc), http.MethodGet, "/api/v1/tasks/recurring/7/instances", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var tasks []map[string]any
	decodeBody(t, rr, &tasks)
	if len(tasks) != 1 {
		t.Errorf("want 1 task, got %d", len(tasks))
	}
	if tasks[0]["recurrence_id"] == nil {
		t.Error("recurrence_id missing from instance")
	}
}

func TestRecurrenceHandler_ListInstances_InternalError(t *testing.T) {
	uc := &mockRecurrenceUsecase{
		listInstancesFn: func(_ context.Context, _ int64) ([]taskdomain.Task, error) {
			return nil, errors.New("db failure")
		},
	}
	rr := do(t, buildRecurrenceRouter(uc), http.MethodGet, "/api/v1/tasks/recurring/1/instances", nil)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", rr.Code)
	}
}
