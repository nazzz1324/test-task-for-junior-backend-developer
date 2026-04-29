package handlers

import (
	"errors"
	"net/http"

	domain "example.com/taskservice/internal/domain/recurrence"
	recurrenceusecase "example.com/taskservice/internal/usecase/recurrence"
)

type RecurrenceHandler struct {
	usecase recurrenceusecase.Usecase
}

func NewRecurrenceHandler(usecase recurrenceusecase.Usecase) *RecurrenceHandler {
	return &RecurrenceHandler{usecase: usecase}
}

// POST /api/v1/tasks/recurring
func (h *RecurrenceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRecurrenceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	result, err := h.usecase.Create(r.Context(), req.toCreateInput())
	if err != nil {
		writeRecurrenceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, createRecurrenceResponse{
		Rule:      newRuleDTO(result.Rule),
		Instances: toTaskDTOs(result.Instances),
	})
}

// GET /api/v1/tasks/recurring
func (h *RecurrenceHandler) List(w http.ResponseWriter, r *http.Request) {
	rules, err := h.usecase.List(r.Context())
	if err != nil {
		writeRecurrenceError(w, err)
		return
	}

	dtos := make([]ruleDTO, len(rules))
	for i := range rules {
		dtos[i] = newRuleDTO(rules[i])
	}
	writeJSON(w, http.StatusOK, dtos)
}

// GET /api/v1/tasks/recurring/{id}
func (h *RecurrenceHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	rule, err := h.usecase.GetByID(r.Context(), id)
	if err != nil {
		writeRecurrenceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, newRuleDTO(*rule))
}

// PUT /api/v1/tasks/recurring/{id}
func (h *RecurrenceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	var req updateRecurrenceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	result, err := h.usecase.Update(r.Context(), id, req.toUpdateInput())
	if err != nil {
		writeRecurrenceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, updateRecurrenceResponse{
		Rule:      newRuleDTO(result.Rule),
		Instances: toTaskDTOs(result.Instances),
	})
}

// DELETE /api/v1/tasks/recurring/{id}
func (h *RecurrenceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := h.usecase.Delete(r.Context(), id); err != nil {
		writeRecurrenceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/tasks/recurring/{id}/instances
func (h *RecurrenceHandler) ListInstances(w http.ResponseWriter, r *http.Request) {
	id, err := getIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	tasks, err := h.usecase.ListInstances(r.Context(), id)
	if err != nil {
		writeRecurrenceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toTaskDTOs(tasks))
}

func writeRecurrenceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, recurrenceusecase.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}
