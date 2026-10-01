-- Archived tickets remain available for audit and can be restored. They are
-- excluded from active ticket and client-story views.
ALTER TABLE tickets ADD COLUMN archived_at TIMESTAMPTZ;

CREATE INDEX tickets_active_project_idx
  ON tickets(project_id, number DESC)
  WHERE archived_at IS NULL;
