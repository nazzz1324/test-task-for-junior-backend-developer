package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	taskdomain "example.com/taskservice/internal/domain/task"
	"example.com/taskservice/internal/transport/http/handlers"
	taskusecase "example.com/taskservice/internal/usecase/task"
)

// ── mock task Usecase ─────────────────────────────────────────────────────────

type mockTaskUsecase struct {
	createFn  func(ctx context.Context, input taskusecase.CreateInput) (*taskdomain.Task, error)
	getByIDFn func(ctx context.Context, id int64) (*taskdomain.Task, error)
	updateFn  func(ctx context.Context, id int64, input taskusecase.UpdateInput) (*taskdomain.Task, error)
	deleteFn  func(ctx context.Context, id int64) error
	listFn    func(ctx context.Context) ([]taskdomain.Task, error)
}

func (m *mockTaskUsecase) Create(ctx context.Context, input taskusecase.CreateInput) (*taskdomain.Task, error) {
	return m.createFn(ctx, input)
}
func (m *mockTaskUsecase) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockTaskUsecase) Update(ctx context.Context, id int64, input taskusecase.UpdateInput) (*taskdomain.Task, error) {
	return m.updateFn(ctx, id, input)
}
func (m *mockTaskUsecase) Delete(ctx context.Context, id int64) error {
	return m.deleteFn(ctx, id)
}
func (m *mockTaskUsecase) List(ctx context.Context) ([]taskdomain.Task, error) {
	return m.listFn(ctx)
}

// ── router helper ─────────────────────────────────────────────────────────────

func buildTaskRouter(uc taskusecase.Usecase) *mux.Router {
	h := handlers.NewTaskHandler(uc)
	r := mux.NewRouter()
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/tasks", h.Create).Methods(http.MethodPost)
	api.HandleFunc("/tasks", h.List).Methods(http.MethodGet)
	api.HandleFunc("/tasks/{id:[0-9]+}", h.GetByID).Methods(http.MethodGet)
	api.HandleFunc("/tasks/{id:[0-9]+}", h.Update).Methods(http.MethodPut)
	api.HandleFunc("/tasks/{id:[0-9]+}", h.Delete).Methods(http.MethodDelete)
	return r
}

func now() time.Time { return time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC) }

// ── POST /api/v1/tasks ────────────────────────────────────────────────────────

func TestTaskHandler_Create_Success(t *testing.T) {
	uc := &mockTaskUsecase{
		createFn: func(_ context.Context, input taskusecase.CreateInput) (*taskdomain.Task, error) {
			return &taskdomain.Task{
				ID: 1, Title: input.Title, Description: input.Description,
				Status: taskdomain.StatusNew, CreatedAt: now(), UpdatedAt: now(),
			}, nil
		},
	}

	body := `{"title":"Buy milk","description":"2% fat","status":"new"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	buildTaskRouter(uc).ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["id"] == nil {
		t.Error("id missing in response")
	}
	if resp["title"] != "Buy milk" {
		t.Errorf("title: want 'Buy milk', got %v", resp["title"])
	}
	// backward compat: recurrence fields must be absent when nil
	if _, ok := resp["recurrence_id"]; ok {
		t.Error("recurrence_id must be omitted for non-recurring tasks")
	}
	if _, ok := resp["scheduled_date"]; ok {
		t.Error("scheduled_date must be omitted for non-recurring tasks")
	}
}

func TestTaskHandler_Create_InvalidJSON(t *testing.T) {
	uc := &mockTaskUsecase{}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	buildTaskRouter(uc).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestTaskHandler_Create_UnknownFieldRejected(t *testing.T) {
	uc := &mockTaskUsecase{}
	body := `{"title":"t","unknown_field":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	buildTaskRouter(uc).ServeHTTP(rr, req)

	// DisallowUnknownFields → 400
	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

func TestTaskHandler_Create_ValidationError(t *testing.T) {
	uc := &mockTaskUsecase{
		createFn: func(_ context.Context, _ taskusecase.CreateInput) (*taskdomain.Task, error) {
			return nil, taskusecase.ErrInvalidInput
		},
	}
	body := `{"title":"","status":"new"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	buildTaskRouter(uc).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rr.Code)
	}
}

// ── GET /api/v1/tasks ─────────────────────────────────────────────────────────

func TestTaskHandler_List_ReturnsTasks(t *testing.T) {
	uc := &mockTaskUsecase{
		listFn: func(_ context.Context) ([]taskdomain.Task, error) {
			return []taskdomain.Task{
				{ID: 1, Title: "Task A", Status: taskdomain.StatusNew, CreatedAt: now(), UpdatedAt: now()},
				{ID: 2, Title: "Task B", Status: taskdomain.StatusDone, CreatedAt: now(), UpdatedAt: now()},
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	rr := httptest.NewRecorder()
	buildTaskRouter(uc).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}

	var tasks []map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&tasks); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("want 2 tasks, got %d", len(tasks))
	}
	// recurrence fields omitted for regular tasks
	for i, task := range tasks {
		if _, ok := task["recurrence_id"]; ok {
			t.Errorf("tasks[%d]: recurrence_id must be omitted", i)
		}
	}
}

func TestTaskHandler_List_Empty(t *testing.T) {
	uc := &mockTaskUsecase{
		listFn: func(_ context.Context) ([]taskdomain.Task, error) {
			return []taskdomain.Task{}, nil
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
	rr := httptest.NewRecorder()
	buildTaskRouter(uc).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rr.Code)
	}
	body := strings.TrimSpace(rr.Body.String())
	if body != "[]" {
		t.Errorf("want empty JSON array, got %s", body)
	}
}

// ── GET /api/v1/tasks/{id} ────────────────────────────────────────────────────

func TestTaskHandler_GetByID_Found(t *testing.T) {
	uc := &mockTaskUsecase{
		getByIDFn: func(_ context.Context, id int64) (*taskdomain.Task, error) {
			return &taskdomain.Task{
				ID: id, Title: "Found task", Status: taskdomain.StatusInProgress,
				CreatedAt: now(), UpdatedAt: now(),
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/42", nil)
	rr := httptest.NewRecorder()
	buildTaskRouter(uc).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var task map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&task); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if task["status"] != "in_progress" {
		t.Errorf("status: want 'in_progress', got %v", task["status"])
	}
}

func TestTaskHandler_GetByID_NotFound(t *testing.T) {
	uc := &mockTaskUsecase{
		getByIDFn: func(_ context.Context, _ int64) (*taskdomain.Task, error) {
			return nil, taskdomain.ErrNotFound
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/999", nil)
	rr := httptest.NewRecorder()
	buildTaskRouter(uc).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}

	var errResp map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if errResp["error"] == nil {
		t.Error("error field missing in 404 response")
	}
}

// ── PUT /api/v1/tasks/{id} ────────────────────────────────────────────────────

func TestTaskHandler_Update_Success(t *testing.T) {
	uc := &mockTaskUsecase{
		updateFn: func(_ context.Context, id int64, input taskusecase.UpdateInput) (*taskdomain.Task, error) {
			return &taskdomain.Task{
				ID: id, Title: input.Title, Description: input.Description,
				Status: input.Status, CreatedAt: now(), UpdatedAt: now(),
			}, nil
		},
	}

	body := `{"title":"Updated","description":"new desc","status":"done"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/tasks/7", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	buildTaskRouter(uc).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", rr.Code, rr.Body.String())
	}

	var task map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&task); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if task["title"] != "Updated" {
		t.Errorf("title: want 'Updated', got %v", task["title"])
	}
	if task["status"] != "done" {
		t.Errorf("status: want 'done', got %v", task["status"])
	}
}

func TestTaskHandler_Update_NotFound(t *testing.T) {
	uc := &mockTaskUsecase{
		updateFn: func(_ context.Context, _ int64, _ taskusecase.UpdateInput) (*taskdomain.Task, error) {
			return nil, taskdomain.ErrNotFound
		},
	}
	body := `{"title":"x","status":"new"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/tasks/999", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	buildTaskRouter(uc).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

// ── DELETE /api/v1/tasks/{id} ─────────────────────────────────────────────────

func TestTaskHandler_Delete_Success(t *testing.T) {
	uc := &mockTaskUsecase{
		deleteFn: func(_ context.Context, _ int64) error { return nil },
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/3", nil)
	rr := httptest.NewRecorder()
	buildTaskRouter(uc).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", rr.Code)
	}
}

func TestTaskHandler_Delete_NotFound(t *testing.T) {
	uc := &mockTaskUsecase{
		deleteFn: func(_ context.Context, _ int64) error { return taskdomain.ErrNotFound },
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/999", nil)
	rr := httptest.NewRecorder()
	buildTaskRouter(uc).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rr.Code)
	}
}

// ── backward compat: recurring task fields appear when set ────────────────────

func TestTaskHandler_GetByID_RecurringTask_FieldsPresent(t *testing.T) {
	recID := int64(5)
	scheduled := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)

	uc := &mockTaskUsecase{
		getByIDFn: func(_ context.Context, id int64) (*taskdomain.Task, error) {
			return &taskdomain.Task{
				ID: id, Title: "Recurring task", Status: taskdomain.StatusNew,
				RecurrenceID:  &recID,
				ScheduledDate: &scheduled,
				CreatedAt: now(), UpdatedAt: now(),
			}, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/10", nil)
	rr := httptest.NewRecorder()
	buildTaskRouter(uc).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}

	var task map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&task); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if task["recurrence_id"] == nil {
		t.Error("recurrence_id must be present for recurring task")
	}
	if task["scheduled_date"] == nil {
		t.Error("scheduled_date must be present for recurring task")
	}
}
