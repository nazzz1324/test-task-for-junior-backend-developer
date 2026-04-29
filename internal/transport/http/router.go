package transporthttp

import (
	"net/http"

	"github.com/gorilla/mux"

	swaggerdocs "example.com/taskservice/internal/transport/http/docs"
	httphandlers "example.com/taskservice/internal/transport/http/handlers"
)

func NewRouter(
	taskHandler *httphandlers.TaskHandler,
	recurrenceHandler *httphandlers.RecurrenceHandler,
	docsHandler *swaggerdocs.Handler,
) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/swagger/openapi.json", docsHandler.ServeSpec).Methods(http.MethodGet)
	router.HandleFunc("/swagger/", docsHandler.ServeUI).Methods(http.MethodGet)
	router.HandleFunc("/swagger", docsHandler.RedirectToUI).Methods(http.MethodGet)

	api := router.PathPrefix("/api/v1").Subrouter()

	// Task CRUD — backward-compatible, unchanged behaviour.
	api.HandleFunc("/tasks", taskHandler.Create).Methods(http.MethodPost)
	api.HandleFunc("/tasks", taskHandler.List).Methods(http.MethodGet)
	api.HandleFunc("/tasks/{id:[0-9]+}", taskHandler.GetByID).Methods(http.MethodGet)
	api.HandleFunc("/tasks/{id:[0-9]+}", taskHandler.Update).Methods(http.MethodPut)
	api.HandleFunc("/tasks/{id:[0-9]+}", taskHandler.Delete).Methods(http.MethodDelete)

	// Recurrence series management.
	// Note: /tasks/recurring must be registered before /tasks/{id} so gorilla/mux
	// matches the literal path segment first.
	api.HandleFunc("/tasks/recurring", recurrenceHandler.Create).Methods(http.MethodPost)
	api.HandleFunc("/tasks/recurring", recurrenceHandler.List).Methods(http.MethodGet)
	api.HandleFunc("/tasks/recurring/{id:[0-9]+}", recurrenceHandler.GetByID).Methods(http.MethodGet)
	api.HandleFunc("/tasks/recurring/{id:[0-9]+}", recurrenceHandler.Update).Methods(http.MethodPut)
	api.HandleFunc("/tasks/recurring/{id:[0-9]+}", recurrenceHandler.Delete).Methods(http.MethodDelete)
	api.HandleFunc("/tasks/recurring/{id:[0-9]+}/instances", recurrenceHandler.ListInstances).Methods(http.MethodGet)

	return router
}
