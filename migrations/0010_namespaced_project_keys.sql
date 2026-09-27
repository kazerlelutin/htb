-- Allow project keys to optionally include a namespace prefix (e.g., ALICE/SITE).
-- The total length must not exceed 40 characters; each part follows the existing format.
ALTER TABLE projects DROP CONSTRAINT IF EXISTS project_key_format;
ALTER TABLE projects ADD CONSTRAINT project_key_format CHECK (
    key = upper(key)
    AND key ~ '^[A-Z][A-Z0-9_]{1,19}(/[A-Z][A-Z0-9_]{1,19})?$'
    AND length(key) <= 40
);
