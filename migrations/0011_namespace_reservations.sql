-- Reserve namespace prefixes to their owning accounts and make their quota a
-- plan capability. NULL max_namespaces represents an unlimited paid plan.
ALTER TABLE plans ADD COLUMN IF NOT EXISTS max_namespaces INTEGER;
ALTER TABLE plans ADD CONSTRAINT plans_max_namespaces_valid
  CHECK (max_namespaces IS NULL OR max_namespaces >= 0);
UPDATE plans SET max_namespaces = 1 WHERE code = 'community' AND max_namespaces IS NULL;

CREATE TABLE namespaces (
  name TEXT PRIMARY KEY,
  owner_user_id BIGINT NOT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT namespace_name_format CHECK (name = upper(name) AND name ~ '^[A-Z][A-Z0-9_]{1,19}$')
);
CREATE INDEX namespaces_owner_idx ON namespaces(owner_user_id);

-- Existing namespaced projects are adopted by their recorded owner. Refuse an
-- ambiguous legacy state rather than silently giving one account another
-- account's namespace.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM projects
    WHERE position('/' IN key) > 0 AND owner_user_id IS NULL
  ) THEN
    RAISE EXCEPTION 'cannot reserve existing namespaces without project owners';
  END IF;
  IF EXISTS (
    SELECT split_part(key, '/', 1)
    FROM projects
    WHERE position('/' IN key) > 0
    GROUP BY split_part(key, '/', 1)
    HAVING count(DISTINCT owner_user_id) > 1
  ) THEN
    RAISE EXCEPTION 'cannot reserve a namespace owned by multiple accounts';
  END IF;
END $$;

INSERT INTO namespaces(name, owner_user_id)
SELECT split_part(key, '/', 1), min(owner_user_id)
FROM projects
WHERE position('/' IN key) > 0
GROUP BY split_part(key, '/', 1)
ON CONFLICT (name) DO NOTHING;
