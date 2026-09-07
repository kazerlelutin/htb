CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id BIGSERIAL PRIMARY KEY,
  zitadel_subject TEXT UNIQUE,
  name TEXT NOT NULL,
  email TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE credentials (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id),
  zitadel_subject TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('human', 'service')),
  max_role TEXT NOT NULL CHECK (max_role IN ('read', 'write', 'admin')),
  disabled_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE projects (
  id BIGSERIAL PRIMARY KEY,
  key TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  archived_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT project_key_format CHECK (key = upper(key) AND key ~ '^[A-Z][A-Z0-9_]{1,19}$')
);

CREATE TABLE project_memberships (
  project_id BIGINT NOT NULL REFERENCES projects(id),
  user_id BIGINT NOT NULL REFERENCES users(id),
  role TEXT NOT NULL CHECK (role IN ('read', 'write', 'admin')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (project_id, user_id)
);

CREATE TABLE features (
  id BIGSERIAL PRIMARY KEY,
  project_id BIGINT NOT NULL REFERENCES projects(id),
  key TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  archived_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (project_id, key)
);

CREATE TABLE tickets (
  id BIGSERIAL PRIMARY KEY,
  project_id BIGINT NOT NULL REFERENCES projects(id),
  parent_ticket_id BIGINT REFERENCES tickets(id),
  related_ticket_id BIGINT REFERENCES tickets(id),
  feature_id BIGINT REFERENCES features(id),
  type TEXT NOT NULL CHECK (type IN ('user_story', 'technical_task', 'bug', 'incident')),
  title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 240),
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL CHECK (status IN ('open', 'in_progress', 'review', 'blocked', 'done')) DEFAULT 'open',
  priority TEXT NOT NULL CHECK (priority IN ('low', 'normal', 'high', 'urgent')) DEFAULT 'normal',
  assignee_user_id BIGINT REFERENCES users(id),
  claimed_by_credential_id BIGINT REFERENCES credentials(id),
  claimed_at TIMESTAMPTZ,
  version INTEGER NOT NULL DEFAULT 1,
  created_by_credential_id BIGINT REFERENCES credentials(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  closed_at TIMESTAMPTZ
);
CREATE INDEX tickets_project_idx ON tickets(project_id, id DESC);
CREATE INDEX tickets_parent_idx ON tickets(parent_ticket_id);
CREATE INDEX tickets_feature_idx ON tickets(feature_id);

CREATE TABLE labels (
  id BIGSERIAL PRIMARY KEY,
  project_id BIGINT NOT NULL REFERENCES projects(id),
  name TEXT NOT NULL,
  UNIQUE (project_id, name)
);
CREATE TABLE ticket_labels (
  ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
  label_id BIGINT NOT NULL REFERENCES labels(id),
  PRIMARY KEY (ticket_id, label_id)
);

CREATE TABLE comments (
  id BIGSERIAL PRIMARY KEY,
  ticket_id BIGINT NOT NULL REFERENCES tickets(id),
  body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 20000),
  author_credential_id BIGINT REFERENCES credentials(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ticket_revisions (
  id BIGSERIAL PRIMARY KEY,
  ticket_id BIGINT NOT NULL REFERENCES tickets(id),
  version INTEGER NOT NULL,
  snapshot JSONB NOT NULL,
  author_credential_id BIGINT REFERENCES credentials(id),
  reason TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (ticket_id, version)
);

CREATE TABLE activity_events (
  id BIGSERIAL PRIMARY KEY,
  project_id BIGINT REFERENCES projects(id),
  ticket_id BIGINT REFERENCES tickets(id),
  action TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  actor_credential_id BIGINT REFERENCES credentials(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE invitations (
  id BIGSERIAL PRIMARY KEY,
  project_id BIGINT NOT NULL REFERENCES projects(id),
  role TEXT NOT NULL CHECK (role IN ('read', 'write', 'admin')),
  code_hash BYTEA NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  accepted_by_user_id BIGINT REFERENCES users(id),
  accepted_at TIMESTAMPTZ,
  created_by_credential_id BIGINT REFERENCES credentials(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE plans (
  code TEXT PRIMARY KEY,
  name TEXT NOT NULL
);
CREATE TABLE plan_features (
  plan_code TEXT NOT NULL REFERENCES plans(code),
  feature_key TEXT NOT NULL,
  PRIMARY KEY (plan_code, feature_key)
);
CREATE TABLE installation_plan (
  singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
  plan_code TEXT NOT NULL REFERENCES plans(code),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO plans(code, name) VALUES ('community', 'Community') ON CONFLICT DO NOTHING;
INSERT INTO installation_plan(singleton, plan_code) VALUES (true, 'community') ON CONFLICT DO NOTHING;
