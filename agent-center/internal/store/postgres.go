package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"agent-center/internal/agent"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const createTableSQL = `
CREATE TABLE IF NOT EXISTS registered_agent (
  name TEXT PRIMARY KEY,
  agent_type TEXT NOT NULL DEFAULT 'custom',
  source TEXT NOT NULL DEFAULT 'local',
  description TEXT NOT NULL DEFAULT '',
  key_name TEXT NOT NULL DEFAULT '',
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  tools TEXT[] NOT NULL DEFAULT '{}',
  runtime_url TEXT NOT NULL DEFAULT '',
  workspace_path TEXT NOT NULL DEFAULT '',
  entrypoint TEXT NOT NULL DEFAULT '',
  owner_name TEXT NOT NULL DEFAULT '',
  tags TEXT[] NOT NULL DEFAULT '{}',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  status TEXT NOT NULL DEFAULT 'registered',
  last_seen_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_registered_agent_enabled ON registered_agent(enabled, name);
CREATE INDEX IF NOT EXISTS idx_registered_agent_updated_at ON registered_agent(updated_at DESC);
`

const addDefaultColumnSQL = `
ALTER TABLE registered_agent
ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT FALSE
`

const createDefaultIndexSQL = `
CREATE UNIQUE INDEX IF NOT EXISTS idx_registered_agent_single_default
ON registered_agent ((1)) WHERE is_default
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
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, createTableSQL); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, addDefaultColumnSQL); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, createDefaultIndexSQL); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) UpsertAgent(ctx context.Context, item agent.RegisteredAgent) (agent.RegisteredAgent, error) {
	item = NormalizeAgent(item)
	if item.Name == "" {
		return agent.RegisteredAgent{}, fmt.Errorf("agent name is required")
	}
	metadata, err := json.Marshal(item.Metadata)
	if err != nil {
		return agent.RegisteredAgent{}, fmt.Errorf("marshal metadata: %w", err)
	}
	if item.IsDefault {
		if _, err := s.pool.Exec(ctx, `UPDATE registered_agent SET is_default = FALSE, updated_at = NOW() WHERE is_default = TRUE AND name <> $1`, item.Name); err != nil {
			return agent.RegisteredAgent{}, fmt.Errorf("clear default agent: %w", err)
		}
	}

	row := s.pool.QueryRow(ctx, `
INSERT INTO registered_agent (
  name, agent_type, source, description, key_name, is_default, tools, runtime_url, workspace_path, entrypoint,
  owner_name, tags, metadata, enabled, status, last_seen_at, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW(), NOW())
ON CONFLICT (name) DO UPDATE SET
  agent_type = EXCLUDED.agent_type,
  source = EXCLUDED.source,
  description = EXCLUDED.description,
  key_name = EXCLUDED.key_name,
  is_default = EXCLUDED.is_default,
  tools = EXCLUDED.tools,
  runtime_url = EXCLUDED.runtime_url,
  workspace_path = EXCLUDED.workspace_path,
  entrypoint = EXCLUDED.entrypoint,
  owner_name = EXCLUDED.owner_name,
  tags = EXCLUDED.tags,
  metadata = EXCLUDED.metadata,
  enabled = EXCLUDED.enabled,
  status = EXCLUDED.status,
  last_seen_at = COALESCE(EXCLUDED.last_seen_at, registered_agent.last_seen_at),
  updated_at = NOW()
RETURNING name, agent_type, source, description, key_name, is_default, tools, runtime_url, workspace_path, entrypoint,
          owner_name, tags, metadata, enabled, status, last_seen_at, created_at, updated_at
`,
		item.Name,
		item.Type,
		item.Source,
		item.Description,
		item.KeyName,
		item.IsDefault,
		item.Tools,
		item.RuntimeURL,
		item.WorkspacePath,
		item.Entrypoint,
		item.Owner,
		item.Tags,
		metadata,
		item.Enabled,
		item.Status,
		item.LastSeenAt,
	)
	return scanAgentRow(row.Scan)
}

func (s *PostgresStore) ListAgents(ctx context.Context, enabledOnly bool) ([]agent.RegisteredAgent, error) {
	query := `
SELECT name, agent_type, source, description, key_name, is_default, tools, runtime_url, workspace_path, entrypoint,
       owner_name, tags, metadata, enabled, status, last_seen_at, created_at, updated_at
FROM registered_agent
`
	args := []any{}
	if enabledOnly {
		query += `WHERE enabled = TRUE `
	}
	query += `ORDER BY is_default DESC, updated_at DESC, name ASC`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]agent.RegisteredAgent, 0)
	for rows.Next() {
		item, scanErr := scanAgentRow(rows.Scan)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *PostgresStore) GetAgent(ctx context.Context, name string) (agent.RegisteredAgent, error) {
	row := s.pool.QueryRow(ctx, `
SELECT name, agent_type, source, description, key_name, is_default, tools, runtime_url, workspace_path, entrypoint,
       owner_name, tags, metadata, enabled, status, last_seen_at, created_at, updated_at
FROM registered_agent
WHERE name = $1
`, NormalizeAgent(agent.RegisteredAgent{Name: name}).Name)
	item, err := scanAgentRow(row.Scan)
	if err != nil {
		if isNotFoundErr(err) {
			return agent.RegisteredAgent{}, ErrNotFound
		}
		return agent.RegisteredAgent{}, err
	}
	return item, nil
}

func (s *PostgresStore) DeleteAgent(ctx context.Context, name string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM registered_agent WHERE name = $1`, NormalizeAgent(agent.RegisteredAgent{Name: name}).Name)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (s *PostgresStore) TouchHeartbeat(ctx context.Context, name string, now time.Time, status string) (agent.RegisteredAgent, error) {
	normalized := NormalizeAgent(agent.RegisteredAgent{Name: name})
	heartbeatStatus := NormalizeHeartbeatStatus(status)
	row := s.pool.QueryRow(ctx, `
UPDATE registered_agent
SET last_seen_at = $2,
    status = $3,
    updated_at = NOW()
WHERE name = $1
RETURNING name, agent_type, source, description, key_name, is_default, tools, runtime_url, workspace_path, entrypoint,
          owner_name, tags, metadata, enabled, status, last_seen_at, created_at, updated_at
`, normalized.Name, now.UTC(), heartbeatStatus)
	item, err := scanAgentRow(row.Scan)
	if err != nil {
		if isNotFoundErr(err) {
			return agent.RegisteredAgent{}, ErrNotFound
		}
		return agent.RegisteredAgent{}, err
	}
	return item, nil
}

type scanFunc func(dest ...any) error

func scanAgentRow(scan scanFunc) (agent.RegisteredAgent, error) {
	var item agent.RegisteredAgent
	var metadataBytes []byte
	var lastSeenAt *time.Time
	if err := scan(
		&item.Name,
		&item.Type,
		&item.Source,
		&item.Description,
		&item.KeyName,
		&item.IsDefault,
		&item.Tools,
		&item.RuntimeURL,
		&item.WorkspacePath,
		&item.Entrypoint,
		&item.Owner,
		&item.Tags,
		&metadataBytes,
		&item.Enabled,
		&item.Status,
		&lastSeenAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return agent.RegisteredAgent{}, err
	}
	if lastSeenAt != nil {
		t := lastSeenAt.UTC()
		item.LastSeenAt = &t
	}
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &item.Metadata); err != nil {
			return agent.RegisteredAgent{}, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	if item.Metadata == nil {
		item.Metadata = map[string]string{}
	}
	return item, nil
}

func isNotFoundErr(err error) bool {
	return err == pgx.ErrNoRows
}
