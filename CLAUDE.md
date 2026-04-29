# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# Backend service - Medods Task Tracker

Go backend service for managing tasks with HTTP API. This project is a test assignment focused on adding recurrence functionality to tasks.

## Commands

- `go run ./cmd/api` — start the API service locally (requires a running PostgreSQL)
- `go test ./...` — run all tests
- `go test ./internal/usecase/task/...` — run tests for a single package
- `go build ./...` — verify the project compiles
- `docker compose up --build` — build and start the service with PostgreSQL
- `docker compose down -v` — remove containers and volumes (required for a clean DB state, since migrations run only on volume init)

## Environment Variables

| Variable       | Default                                                              |
|----------------|----------------------------------------------------------------------|
| `HTTP_ADDR`    | `:8080`                                                              |
| `DATABASE_DSN` | `postgres://postgres:postgres@localhost:5432/taskservice?sslmode=disable` |

## Architecture

The project uses clean architecture with four internal layers:

```
cmd/api/                          → wiring + HTTP server lifecycle
internal/
  domain/task/                    → Task model, Status enum, ErrNotFound
  usecase/task/                   → business logic (Service), input types, Repository/Usecase interfaces (ports.go)
  repository/postgres/            → pgx/v5 implementation of usecase.Repository
  infrastructure/postgres/        → connection pool (pgxpool.Open wrapper)
  transport/http/                 → gorilla/mux router
  transport/http/handlers/        → TaskHandler, DTOs, JSON helpers
  transport/http/docs/            → embedded OpenAPI spec + Swagger UI handler
migrations/                       → SQL migration (applied automatically via docker-entrypoint-initdb.d)
```

Dependency flow: `transport → usecase (interface) ← repository`. The `usecase/task/ports.go` file defines the `Repository` and `Usecase` interfaces that decouple layers.

### Key Design Points

- **Error mapping**: `taskdomain.ErrNotFound` → 404, `taskusecase.ErrInvalidInput` → 400, everything else → 500. Mapping lives in `writeUsecaseError` in `transport/http/handlers/task_handler.go`.
- **Testable time**: `Service.now` is an injected `func() time.Time` (defaults to `time.Now().UTC()`), allowing deterministic tests without mocks on the clock.
- **JSON decoding**: `DisallowUnknownFields()` is used — unknown JSON fields in requests return 400.
- **Swagger**: served at `http://localhost:8080/swagger/`; spec at `/swagger/openapi.json`. The spec file is embedded at compile time from `internal/transport/http/docs/openapi.json`.
- **Migrations**: only an up migration exists (`migrations/0001_create_tasks.up.sql`); no down migration.

## Rules

- Use `context.Context` for all repository and usecase calls.
- Wrap errors with `%w`.
- Validate input in the usecase layer (business rules), not in handlers. Handlers only decode/encode and delegate.
- Prefer table-driven tests.
- Keep handlers thin — no business logic.

## Notes

- `HISTORY.md` must be maintained to document interactions with LLMs.
- Reasonable assumptions should be documented in `README.md`.
- The primary goal is to implement task recurrence functionality as described in the test assignment.
