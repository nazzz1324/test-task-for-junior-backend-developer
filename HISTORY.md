# HISTORY.md

История взаимодействий с LLM в рамках реализации фичи повторяющихся задач.

---

## Сессия 1 — 2026-04-28

### Промт 1 — Архитектурный дизайн

> Изучи код проекта (Go, слои: handlers, service, repository, migrations).
> Предложи полную архитектуру фичи для добавления повторяющихся задач: ежедневно каждые N дней, ежемесячно на число (1–30), по списку конкретных дат, только по чётным/нечётным дням месяца.

**Ответ (резюме):**

Проведён анализ существующей кодовой базы (clean architecture: `domain → usecase → repository → transport`). Предложена следующая архитектура:

- **Отдельная таблица `task_recurrences`** для хранения правил повторения; задачи-экземпляры хранятся в существующей таблице `tasks` с добавлением полей `recurrence_id` и `scheduled_date`.
- **Domain-слой** (`internal/domain/recurrence/`): тип `Rule` с пятью типами повторения (`every_n_days`, `monthly_on_day`, `specific_dates`, `even_days`, `odd_days`).
- **Usecase-слой** (`internal/usecase/recurrence/`): чистая функция `NextDates` для вычисления дат, сервис с методами Create/Update/Delete/ExtendHorizon, планировщик-горутина.
- **Стратегия генерации**: eager pre-generation с горизонтом 90 дней; планировщик ежедневно продлевает горизонт.
- **Логика редактирования**: `type` и `starts_at` иммутабельны; UPDATE удаляет будущие экземпляры и регенерирует их.
- **Логика удаления**: прошлые экземпляры детачатся (`recurrence_id = NULL`), будущие удаляются CASCADE.
- **HTTP**: новые эндпоинты `/api/v1/tasks/recurring/...`; существующие `/api/v1/tasks/...` без изменений.

- `HISTORY.md` (этот файл) — создан.
---

### Промт 2 — Реализация

> Реализуй по утверждённой архитектуре:
> - SQL-миграцию (up/down) в стиле проекта,
> - модели и методы репозитория (context.Context, %w),
> - сервис: вычисление ближайшей даты для каждого типа повторения, планировщик (горутина/компонент в cmd/api), логика редактирования и удаления серий (обоснованно),
> - обработчики: расширение POST/GET, новые эндпоинты для управления сериями, валидация и понятные ошибки.
> Сохрани обратную совместимость: задачи без recurrence работают как раньше.
> Код — в стиле проекта, чистый, тестируемый.

**Ответ (резюме):**

Реализованы все слои фичи. Созданы следующие файлы:

| Файл | Назначение |
|------|-----------|
| `migrations/0002_add_recurrence.up.sql` | Таблица `task_recurrences` + поля в `tasks` + индексы |
| `migrations/0002_add_recurrence.down.sql` | Откат миграции |
| `internal/domain/recurrence/rule.go` | Тип `Rule`, enum `Type`, метод `Valid()` |
| `internal/domain/recurrence/errors.go` | `ErrNotFound` |
| `internal/usecase/recurrence/generator.go` | Чистая функция `NextDates` для всех 5 типов |
| `internal/usecase/recurrence/ports.go` | Интерфейсы `Repository`, `TaskRepository`, `Usecase` |
| `internal/usecase/recurrence/service.go` | Бизнес-логика: Create, Update, Delete, ExtendHorizon |
| `internal/usecase/recurrence/scheduler.go` | Горутина, ежедневно продлевает горизонт |
| `internal/usecase/recurrence/errors.go` | `ErrInvalidInput` |
| `internal/repository/postgres/recurrence_repository.go` | CRUD для `task_recurrences` с `pgtype.Array[pgtype.Date]` |
| `internal/transport/http/handlers/recurrence_dto.go` | Request/response DTO + конвертеры |
| `internal/transport/http/handlers/recurrence_handler.go` | 6 HTTP-эндпоинтов с маппингом ошибок |

Изменены:
- `internal/domain/task/task.go` — добавлены `RecurrenceID *int64`, `ScheduledDate *time.Time` (оба `omitempty`)
- `internal/repository/postgres/task_repository.go` — обновлены SQL-запросы и `scanTask`, добавлены `CreateBatch`, `ListByRecurrenceID`, `DeleteFutureByRecurrenceID`, `DetachPastByRecurrenceID`
- `internal/transport/http/handlers/dto.go` — `taskDTO` расширен полями `recurrence_id` и `scheduled_date`
- `internal/transport/http/router.go` — добавлены 6 маршрутов `/api/v1/tasks/recurring/...`
- `cmd/api/main.go` — wiring нового сервиса, репозитория, хендлера и планировщика
- `docker-compose.yml` — монтирование второй миграции

---
## Сессия 2 — 2026-04-29
### Промт 1 — Тесты и документация

> Напиши table-driven модульные тесты для всех функций расчёта дат (включая 31→февраль, несуществующие даты, пропуск дней сервисом).
> Интеграционные тесты API: создание/чтение с recurrence, редактирование/удаление серии. Старые тесты не должны сломаться.
> Дополни README.md: описание фичи, принятые допущения и ограничения.
> Сформируй HISTORY.md с этими промтами и твоими ответами.

**Ответ (резюме):**

Написаны тесты и документация:

- `internal/usecase/recurrence/generator_test.go` — table-driven unit-тесты функции `NextDates`:
  - `every_n_days`: базовый случай, `from` между датами, `from` точно на дате, `from` до `starts_at`, срезание по `ends_at`, `N=30` пересекает границу месяца.
  - `monthly_on_day`: день 31 пропускает фев/апр/июн/сен/ноя; день 30 пропускает февраль (2024 — високосный); день 29 в невисокосный год пропускает февраль; день 29 в 2024 включает 29 февраля; `from` после числа в текущем месяце → переходит к следующему.
  - `specific_dates`: сортировка, фильтрация по `from`, ограничение `maxCount`, срезание по `ends_at`.
  - `even_days` / `odd_days`: базовые случаи, граница месяца (день 30/31), срезание по `ends_at`.

- `internal/usecase/recurrence/service_test.go` — сервисные тесты с mock-репозиториями (внутри пакета для доступа к полю `now`):
  - Create: валидный ввод, пустой заголовок, неверный тип, отсутствующий `interval_days`, `ends_at` раньше `starts_at`.
  - Update: удаляет будущие экземпляры от «завтра», регенерирует; 404 при отсутствии правила.
  - Delete: порядок вызовов (`DetachPast` → `Delete`), `before` = сегодня, ошибка при `id=0`.
  - ExtendHorizon: без существующих экземпляров, с экземплярами (генерация от следующего дня), уже на горизонте (CreateBatch не вызывается).

- `internal/transport/http/handlers/recurrence_handler_test.go` — HTTP-тесты через `httptest` с mock usecase:
  - POST: успех (201), некорректный JSON (400), ошибка валидации юзкейса (400).
  - GET list: успех (200), 2 правила.
  - GET by ID: найден (200), не найден (404).
  - PUT: успех (200), не найден (404).
  - DELETE: успех (204), не найден (404).
  - GET instances: успех (200) с `recurrence_id` в ответе, внутренняя ошибка (500).

- `README.md` — дополнен разделами: таблица типов повторения, пример запроса, логика редактирования/удаления серии, описание планировщика, принятые допущения (8 пунктов), ограничения.

---

### Промт 2 — Доделать тесты (backward compat)

> доделай тесты

**Ответ (резюме):**

Дописан недостающий файл тестов для существующего task handler:

- `internal/transport/http/handlers/task_handler_test.go` (`package handlers_test`) — backward compatibility тесты:
  - **Create**: 201 без recurrence-полей в ответе (`omitempty` скрывает `recurrence_id` и `scheduled_date`); 400 при невалидном JSON; 400 при неизвестном поле (`DisallowUnknownFields`); 400 при ошибке валидации usecase.
  - **List**: 200 с двумя задачами; пустой массив `[]`; `recurrence_id` отсутствует в ответе обычных задач.
  - **GetByID**: 200 с корректным `status`; 404 с полем `error` в теле.
  - **Update**: 200 с обновлёнными `title` и `status`; 404.
  - **Delete**: 204; 404.
  - **Recurring task fields**: отдельный тест — `GET /tasks/{id}` для recurring задачи возвращает `recurrence_id` и `scheduled_date`.

---

### Промт 5 — Дополнить HISTORY.md

> дополни HISTORY.md

**Ответ:** Добавлены промты 4 и 5 в этот файл.
