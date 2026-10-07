-- Invitations can target either a single project or a whole namespace. Keeping
-- one code hash table preserves the global uniqueness property of invite codes.
ALTER TABLE invitations ALTER COLUMN project_id DROP NOT NULL;
ALTER TABLE invitations ADD COLUMN namespace_name TEXT REFERENCES namespaces(name);
ALTER TABLE invitations ADD CONSTRAINT invitations_one_target
  CHECK (num_nonnulls(project_id, namespace_name) = 1);

-- Namespace memberships are applied dynamically to every project carrying the
-- prefix, including projects created after the invitation was accepted.
CREATE TABLE namespace_memberships (
  namespace_name TEXT NOT NULL REFERENCES namespaces(name),
  user_id BIGINT NOT NULL REFERENCES users(id),
  role TEXT NOT NULL CHECK (role IN ('read', 'write', 'admin')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (namespace_name, user_id)
);
CREATE INDEX namespace_memberships_user_idx ON namespace_memberships(user_id);
