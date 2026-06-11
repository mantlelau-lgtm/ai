CREATE TABLE IF NOT EXISTS inbound_event (
  id BIGSERIAL PRIMARY KEY,
  event_id TEXT NOT NULL UNIQUE,
  channel TEXT NOT NULL,
  event_type TEXT NOT NULL,
  chat_id TEXT,
  message_id TEXT,
  sender_id TEXT,
  payload_raw JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS job (
  job_id TEXT PRIMARY KEY,
  job_type TEXT NOT NULL,
  status TEXT NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 8,
  next_run_at TIMESTAMPTZ NOT NULL,
  dedup_key TEXT UNIQUE,
  last_error TEXT NOT NULL DEFAULT '',
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_job_status_next_run_at ON job(status, next_run_at);
