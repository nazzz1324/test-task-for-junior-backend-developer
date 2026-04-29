CREATE TABLE task_recurrences (
    id               BIGSERIAL PRIMARY KEY,
    task_title       TEXT        NOT NULL,
    task_description TEXT        NOT NULL DEFAULT '',
    type             TEXT        NOT NULL,
    interval_days    SMALLINT,
    day_of_month     SMALLINT,
    specific_dates   DATE[],
    starts_at        DATE        NOT NULL,
    ends_at          DATE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE tasks
    ADD COLUMN recurrence_id  BIGINT REFERENCES task_recurrences(id) ON DELETE CASCADE,
    ADD COLUMN scheduled_date DATE;

CREATE INDEX idx_tasks_recurrence_id  ON tasks (recurrence_id);
CREATE INDEX idx_tasks_scheduled_date ON tasks (scheduled_date);
