ALTER TABLE tickets RENAME COLUMN client_visible_at TO client_published_at;
ALTER INDEX tickets_client_visible_idx RENAME TO tickets_client_published_idx;

ALTER TABLE tickets ADD COLUMN client_visibility TEXT NOT NULL DEFAULT 'draft'
  CHECK (client_visibility IN ('draft', 'published'));
ALTER TABLE tickets ADD CONSTRAINT tickets_client_visibility_story_only
  CHECK (type = 'user_story' OR client_visibility = 'draft');

UPDATE tickets
SET client_visibility = 'published'
WHERE client_published_at IS NOT NULL;

CREATE INDEX tickets_client_visibility_idx
  ON tickets(project_id, number DESC)
  WHERE client_visibility = 'published';
