CREATE TABLE web_sessions (
  token_hash BYTEA PRIMARY KEY,
  credential_id BIGINT NOT NULL REFERENCES credentials(id) ON DELETE CASCADE,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX web_sessions_credential_active_idx
  ON web_sessions(credential_id, expires_at)
  WHERE revoked_at IS NULL;
