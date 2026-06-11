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
	cost DOUBLE PRECISION NOT NULL DEFAULT 0,
	success BOOLEAN NOT NULL,
	latency_ms BIGINT NOT NULL DEFAULT 0,
	started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	finished_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	error_message TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`
	const defaultIndex = `
CREATE UNIQUE INDEX IF NOT EXISTS providers_single_default_idx
ON providers ((is_default))
WHERE is_default = TRUE`

	alterUsageTable := `
ALTER TABLE IF EXISTS usage_records
	ADD COLUMN IF NOT EXISTS cost DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE IF EXISTS usage_records
	ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE IF EXISTS usage_records
	ADD COLUMN IF NOT EXISTS finished_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
`

	for _, statement := range []string{providersTable, usageTable, alterUsageTable, defaultIndex} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate statement failed: %w", err)
		}
	}
	return nil
}

func (s *PostgresStore) UpsertProvider(ctx context.Context, provider gateway.ProviderConfig) error {
	if provider.ModelPrefixes == nil {
		provider.ModelPrefixes = []string{}
	}
	if provider.Metadata == nil {
		provider.Metadata = map[string]string{}
	}
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
	request_id, provider, endpoint, model, prompt_tokens, completion_tokens, total_tokens, cost, success, latency_ms, started_at, finished_at, error_message, created_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, NOW())`,
		record.RequestID,
		record.Provider,
		record.Endpoint,
		record.Model,
		record.PromptTokens,
		record.CompletionTokens,
		record.TotalTokens,
		record.Cost,
		record.Success,
		record.LatencyMS,
		record.StartedAt,
		record.FinishedAt,
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
SELECT id, request_id, provider, endpoint, model, prompt_tokens, completion_tokens, total_tokens, cost, success, latency_ms, started_at, finished_at, error_message, created_at
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
			&record.Cost,
			&record.Success,
			&record.LatencyMS,
			&record.StartedAt,
			&record.FinishedAt,
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
	var prefixesRaw any
	var metadataBytes []byte

	if err := scan(
		&provider.Name,
		&provider.Type,
		&provider.BaseURL,
		&provider.APIKey,
		&prefixesRaw,
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
	prefixes, err := parseModelPrefixes(prefixesRaw)
	if err != nil {
		return gateway.ProviderConfig{}, fmt.Errorf("parse provider model_prefixes: %w", err)
	}
	provider.ModelPrefixes = prefixes
	provider.Type = strings.ToLower(provider.Type)
	return provider, nil
}

func parseModelPrefixes(raw any) ([]string, error) {
	switch v := raw.(type) {
	case nil:
		return []string{}, nil
	case []string:
		return sanitizePrefixes(v), nil
	case []byte:
		return parseModelPrefixes(string(v))
	case string:
		s := strings.TrimSpace(v)
		if s == "" || s == "{}" {
			return []string{}, nil
		}
		if strings.HasPrefix(s, "[") {
			var arr []string
			if err := json.Unmarshal([]byte(s), &arr); err != nil {
				return nil, err
			}
			return sanitizePrefixes(arr), nil
		}
		if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			inner := strings.TrimSuffix(strings.TrimPrefix(s, "{"), "}")
			if strings.TrimSpace(inner) == "" {
				return []string{}, nil
			}
			parts := splitPGArray(inner)
			return sanitizePrefixes(parts), nil
		}
		return sanitizePrefixes([]string{s}), nil
	default:
		return nil, fmt.Errorf("unsupported type %T", raw)
	}
}

func sanitizePrefixes(prefixes []string) []string {
	out := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		p := strings.TrimSpace(prefix)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if out == nil {
		return []string{}
	}
	return out
}

func splitPGArray(input string) []string {
	items := []string{}
	var buf strings.Builder
	inQuotes := false
	escaped := false
	for _, r := range input {
		if escaped {
			buf.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && inQuotes {
			escaped = true
			continue
		}
		if r == '"' {
			inQuotes = !inQuotes
			continue
		}
		if r == ',' && !inQuotes {
			items = append(items, buf.String())
			buf.Reset()
			continue
		}
		buf.WriteRune(r)
	}
	items = append(items, buf.String())
	return items
}
