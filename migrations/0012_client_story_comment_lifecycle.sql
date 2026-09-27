ALTER TABLE client_story_comments
  ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
