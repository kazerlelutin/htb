ALTER TABLE projects ADD COLUMN IF NOT EXISTS owner_user_id BIGINT REFERENCES users(id);

-- Existing projects predate explicit ownership. The first project administrator
-- is the only reliable owner available during the migration.
UPDATE projects AS project
SET owner_user_id = (
  SELECT membership.user_id
  FROM project_memberships AS membership
  WHERE membership.project_id = project.id
  ORDER BY membership.created_at, membership.user_id
  LIMIT 1
)
WHERE project.owner_user_id IS NULL;

CREATE INDEX IF NOT EXISTS projects_owner_active_idx
  ON projects(owner_user_id)
  WHERE archived_at IS NULL;

ALTER TABLE plans ADD COLUMN IF NOT EXISTS max_projects INTEGER;
ALTER TABLE plans ADD CONSTRAINT plans_max_projects_valid
  CHECK (max_projects IS NULL OR max_projects >= 0);

UPDATE plans SET max_projects = 3 WHERE code = 'community' AND max_projects IS NULL;

CREATE TABLE user_plans (
  user_id BIGINT PRIMARY KEY REFERENCES users(id),
  plan_code TEXT NOT NULL REFERENCES plans(code),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO user_plans(user_id, plan_code)
SELECT id, 'community' FROM users
ON CONFLICT (user_id) DO NOTHING;
