package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"llm-gateway/internal/gateway"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store interface {
	Migrate(ctx context.Context) error
	UpsertProvider(ctx context.Context, provider gateway.ProviderConfig) error
	DeleteProvider(ctx context.Context, name string) error
	ListProviders(ctx context.Context) ([]gateway.ProviderConfig, error)
	GetProvider(ctx context.Context, name string) (gateway.ProviderConfig, error)
	RecordUsage(ctx context.Context, record gateway.UsageRecord) error
	ListUsage(ctx context.Context, limit int) ([]gateway.UsageRecord, error)
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) Migrate(ctx context.Context) error {
	const providersTable = `
CREATE TABLE IF NOT EXISTS providers (
	name TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	base_url TEXT NOT NULL DEFAULT '',
	api_key TEXT NOT NULL DEFAULT '',
	model_prefixes TEXT[] NOT NULL DEFAULT '{}',
	enabled BOOLEAN NOT NULL DEFAULT TRUE,
	is_default BOOLEAN NOT NULL DEFAULT FALSE,
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`
	const usageTable = `
CREATE TABLE IF NOT EXISTS usage_records (
	id BIGSERIAL PRIMARY KEY,
	request_id TEXT NOT NULL,
	provider TEXT NOT NULL,
	endpoint TEXT NOT NULL,
	model TEXT NOT NULL,
	prompt_tokens INTEGER NOT NULL DEFAULT 0,
	completion_tokens INTEGER NOT NULL DEFAULT 0,
	total_tokens INTEGER NOT NULL DEFAULT 0,
	success BOOLEAN NOT NULL,
	latency_ms BIGINT NOT NULL DEFAULT 0,
	error_message TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`
	const defaultIndex = `
CREATE UNIQUE INDEX IF NOT EXISTS providers_single_default_idx
ON providers ((is_default))
WHERE is_default = TRUE`

	for _, statement := range []string{providersTable, usageTable, defaultIndex} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate statement failed: %w", err)
		}
	}
	return nil
}

func (s *PostgresStore) UpsertProvider(ctx context.Context, provider gateway.ProviderConfig) error {
	metadata, err := json.Marshal(provider.Metadata)
	if err != nil {
		return fmt.Errorf("marshal provider metadata: %w", err)
	}

	if provider.IsDefault {
		if _, err := s.db.ExecContext(ctx, `UPDATE providers SET is_default = FALSE, updated_at = NOW() WHERE name <> $1`, provider.Name); err != nil {
			return fmt.Errorf("clear default provider: %w", err)
		}
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO providers (name, type, base_url, api_key, model_prefixes, enabled, is_default, metadata, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
ON CONFLICT (name) DO UPDATE SET
	type = EXCLUDED.type,
	base_url = EXCLUDED.base_url,
	api_key = EXCLUDED.api_key,
	model_prefixes = EXCLUDED.model_prefixes,
	enabled = EXCLUDED.enabled,
	is_default = EXCLUDED.is_default,
	metadata = EXCLUDED.metadata,
	updated_at = NOW()`,
		provider.Name,
		provider.Type,
		provider.BaseURL,
		provider.APIKey,
		provider.ModelPrefixes,
		provider.Enabled,
		provider.IsDefault,
		metadata,
	)
	if err != nil {
		return fmt.Errorf("upsert provider: %w", err)
	}
	return nil
}

func (s *PostgresStore) DeleteProvider(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM providers WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	return nil
}

func (s *PostgresStore) ListProviders(ctx context.Context) ([]gateway.ProviderConfig, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT name, type, base_url, api_key, model_prefixes, enabled, is_default, metadata, created_at, updated_at
FROM providers
ORDER BY is_default DESC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()

	var providers []gateway.ProviderConfig
	for rows.Next() {
		provider, err := scanProvider(rows.Scan)
		if err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	return providers, rows.Err()
}

func (s *PostgresStore) GetProvider(ctx context.Context, name string) (gateway.ProviderConfig, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT name, type, base_url, api_key, model_prefixes, enabled, is_default, metadata, created_at, updated_at
FROM providers
WHERE name = $1`, name)
	return scanProvider(row.Scan)
}

func (s *PostgresStore) RecordUsage(ctx context.Context, record gateway.UsageRecord) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO usage_records (
	request_id, provider, endpoint, model, prompt_tokens, completion_tokens, total_tokens, success, latency_ms, error_message, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())`,
		record.RequestID,
		record.Provider,
		record.Endpoint,
		record.Model,
		record.PromptTokens,
		record.CompletionTokens,
		record.TotalTokens,
		record.Success,
		record.LatencyMS,
		record.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("record usage: %w", err)
	}
	return nil
}

func (s *PostgresStore) ListUsage(ctx context.Context, limit int) ([]gateway.UsageRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, request_id, provider, endpoint, model, prompt_tokens, completion_tokens, total_tokens, success, latency_ms, error_message, created_at
FROM usage_records
ORDER BY id DESC
LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list usage: %w", err)
	}
	defer rows.Close()

	var records []gateway.UsageRecord
	for rows.Next() {
		var record gateway.UsageRecord
		if err := rows.Scan(
			&record.ID,
			&record.RequestID,
			&record.Provider,
			&record.Endpoint,
			&record.Model,
			&record.PromptTokens,
			&record.CompletionTokens,
			&record.TotalTokens,
			&record.Success,
			&record.LatencyMS,
			&record.ErrorMessage,
			&record.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan usage record: %w", err)
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

type scanFunc func(dest ...any) error

func scanProvider(scan scanFunc) (gateway.ProviderConfig, error) {
	var provider gateway.ProviderConfig
	var metadataBytes []byte

	if err := scan(
		&provider.Name,
		&provider.Type,
		&provider.BaseURL,
		&provider.APIKey,
		&provider.ModelPrefixes,
		&provider.Enabled,
		&provider.IsDefault,
		&metadataBytes,
		&provider.CreatedAt,
		&provider.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return gateway.ProviderConfig{}, fmt.Errorf("provider not found")
		}
		return gateway.ProviderConfig{}, fmt.Errorf("scan provider: %w", err)
	}
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &provider.Metadata); err != nil {
			return gateway.ProviderConfig{}, fmt.Errorf("unmarshal provider metadata: %w", err)
		}
	}
	if provider.Metadata == nil {
		provider.Metadata = map[string]string{}
	}
	provider.Type = strings.ToLower(provider.Type)
	return provider, nil
}
