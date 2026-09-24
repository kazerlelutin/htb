ALTER TABLE tickets ADD COLUMN client_visible_at TIMESTAMPTZ;
ALTER TABLE tickets ADD CONSTRAINT tickets_client_visible_story_only CHECK (client_visible_at IS NULL OR type='user_story');
CREATE INDEX tickets_client_visible_idx ON tickets(project_id, number DESC) WHERE client_visible_at IS NOT NULL;

CREATE TABLE client_story_comments (
  id BIGSERIAL PRIMARY KEY,
  ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
  body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 20000),
  author_credential_id BIGINT NOT NULL REFERENCES credentials(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX client_story_comments_ticket_idx ON client_story_comments(ticket_id, id);
