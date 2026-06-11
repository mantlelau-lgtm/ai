package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"message-gateway/internal/model"
)

const migrationSQL = `
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
`

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *PostgresStore) Migrate(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, migrationSQL)
	return err
}

func (s *PostgresStore) InsertInboundEvent(ctx context.Context, env model.Envelope) (bool, error) {
	const query = `
INSERT INTO inbound_event(event_id, channel, event_type, chat_id, message_id, sender_id, payload_raw)
VALUES ($1, 'lark', $2, $3, $4, $5, $6)
ON CONFLICT (event_id) DO NOTHING
`

	tag, err := s.pool.Exec(ctx, query, env.EventID, env.EventType, env.ChatID, env.MessageID, env.SenderOpenID, env.Raw)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *PostgresStore) EnqueueSendMessage(ctx context.Context, payload model.SendMessagePayload, dedupKey string, maxAttempts int) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	const query = `
INSERT INTO job(job_id, job_type, status, attempts, max_attempts, next_run_at, dedup_key, payload)
VALUES ($1, 'send_message', $2, 0, $3, now(), $4, $5)
ON CONFLICT (dedup_key) DO NOTHING
`

	_, err = s.pool.Exec(ctx, query, newID(), model.JobStatusPending, maxAttempts, dedupKey, body)
	return err
}

func (s *PostgresStore) EnqueueForwardToCore(ctx context.Context, payload model.ForwardToCorePayload, dedupKey string, maxAttempts int) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	const query = `
INSERT INTO job(job_id, job_type, status, attempts, max_attempts, next_run_at, dedup_key, payload)
VALUES ($1, 'forward_to_core', $2, 0, $3, now(), $4, $5)
ON CONFLICT (dedup_key) DO NOTHING
`

	_, err = s.pool.Exec(ctx, query, newID(), model.JobStatusPending, maxAttempts, dedupKey, body)
	return err
}

func (s *PostgresStore) ClaimPendingJobs(ctx context.Context, limit int) ([]model.Job, error) {
	const query = `
WITH picked AS (
  SELECT job_id
  FROM job
  WHERE status = $1 AND next_run_at <= now()
  ORDER BY next_run_at ASC
  LIMIT $2
  FOR UPDATE SKIP LOCKED
)
UPDATE job j
SET status = $3,
    updated_at = now()
FROM picked
WHERE j.job_id = picked.job_id
RETURNING j.job_id, j.job_type, j.status, j.attempts, j.max_attempts, j.payload
`

	rows, err := s.pool.Query(ctx, query, model.JobStatusPending, limit, model.JobStatusRunning)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []model.Job
	for rows.Next() {
		var job model.Job
		if err := rows.Scan(&job.ID, &job.JobType, &job.Status, &job.Attempts, &job.MaxAttempts, &job.Payload); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

func (s *PostgresStore) MarkJobSucceeded(ctx context.Context, jobID string) error {
	const query = `
UPDATE job
SET status = $2, updated_at = now()
WHERE job_id = $1
`
	_, err := s.pool.Exec(ctx, query, jobID, model.JobStatusSucceeded)
	return err
}

func (s *PostgresStore) MarkJobRetry(ctx context.Context, jobID, lastError string, nextRunAt time.Time) error {
	const query = `
UPDATE job
SET status = $2,
    attempts = attempts + 1,
    last_error = $3,
    next_run_at = $4,
    updated_at = now()
WHERE job_id = $1
`
	_, err := s.pool.Exec(ctx, query, jobID, model.JobStatusPending, lastError, nextRunAt)
	return err
}

func (s *PostgresStore) MarkJobDead(ctx context.Context, jobID, lastError string) error {
	const query = `
UPDATE job
SET status = $2,
    attempts = attempts + 1,
    last_error = $3,
    updated_at = now()
WHERE job_id = $1
`
	_, err := s.pool.Exec(ctx, query, jobID, model.JobStatusDead, lastError)
	return err
}

func (s *PostgresStore) ListDeadJobs(ctx context.Context, limit int) ([]model.JobSummary, error) {
	const query = `
SELECT job_id, job_type, status, attempts, max_attempts, last_error, updated_at, payload
FROM job
WHERE status = $1
ORDER BY updated_at DESC
LIMIT $2
`

	rows, err := s.pool.Query(ctx, query, model.JobStatusDead, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.JobSummary
	for rows.Next() {
		var it model.JobSummary
		if err := rows.Scan(&it.JobID, &it.JobType, &it.Status, &it.Attempts, &it.MaxAttempts, &it.LastError, &it.UpdatedAt, &it.Payload); err != nil {
			return nil, err
		}
		items = append(items, it)
	}

	return items, rows.Err()
}

func (s *PostgresStore) ReplayDeadJob(ctx context.Context, jobID string) (bool, error) {
	const query = `
UPDATE job
SET status = $2,
    attempts = 0,
    next_run_at = now(),
    last_error = '',
    updated_at = now()
WHERE job_id = $1 AND status = $3
`
	tag, err := s.pool.Exec(ctx, query, jobID, model.JobStatusPending, model.JobStatusDead)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func newID() string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("job-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}
