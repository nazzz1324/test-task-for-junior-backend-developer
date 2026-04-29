DROP INDEX IF EXISTS idx_tasks_scheduled_date;
DROP INDEX IF EXISTS idx_tasks_recurrence_id;

ALTER TABLE tasks
    DROP COLUMN IF EXISTS scheduled_date,
    DROP COLUMN IF EXISTS recurrence_id;

DROP TABLE IF EXISTS task_recurrences;
