CREATE INDEX IF NOT EXISTS tasks_pending_id_idx ON tasks (id) WHERE status = 'pending';
