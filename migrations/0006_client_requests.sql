CREATE TABLE client_requests (
  id BIGSERIAL PRIMARY KEY,
  project_id BIGINT NOT NULL REFERENCES projects(id),
  title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 240),
  body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 20000),
  status TEXT NOT NULL DEFAULT 'received' CHECK (status IN ('received', 'in_progress', 'needs_info', 'done')),
  linked_ticket_id BIGINT REFERENCES tickets(id),
  submitted_by_credential_id BIGINT NOT NULL REFERENCES credentials(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX client_requests_project_idx ON client_requests(project_id, id DESC);

CREATE TABLE client_request_comments (
  id BIGSERIAL PRIMARY KEY,
  request_id BIGINT NOT NULL REFERENCES client_requests(id) ON DELETE CASCADE,
  body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 20000),
  author_credential_id BIGINT NOT NULL REFERENCES credentials(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX client_request_comments_request_idx ON client_request_comments(request_id, id);
