package store

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"llm-gateway/internal/gateway"
)

type MemoryStore struct {
	mu        sync.RWMutex
	providers map[string]gateway.ProviderConfig
	usage     []gateway.UsageRecord
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		providers: map[string]gateway.ProviderConfig{},
		usage:     []gateway.UsageRecord{},
	}
}

func (s *MemoryStore) Migrate(context.Context) error {
	return nil
}

func (s *MemoryStore) UpsertProvider(_ context.Context, provider gateway.ProviderConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if existing, ok := s.providers[provider.Name]; ok {
		provider.CreatedAt = existing.CreatedAt
	} else {
		provider.CreatedAt = now
	}
	provider.UpdatedAt = now
	if provider.Metadata == nil {
		provider.Metadata = map[string]string{}
	}
	if provider.IsDefault {
		for name, item := range s.providers {
			item.IsDefault = false
			s.providers[name] = item
		}
	}
	s.providers[provider.Name] = provider
	return nil
}

func (s *MemoryStore) DeleteProvider(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.providers, name)
	return nil
}

func (s *MemoryStore) ListProviders(_ context.Context) ([]gateway.ProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]gateway.ProviderConfig, 0, len(s.providers))
	for _, provider := range s.providers {
		items = append(items, provider)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDefault != items[j].IsDefault {
			return items[i].IsDefault
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

func (s *MemoryStore) GetProvider(_ context.Context, name string) (gateway.ProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	provider, ok := s.providers[name]
	if !ok {
		return gateway.ProviderConfig{}, fmt.Errorf("provider not found")
	}
	return provider, nil
}

func (s *MemoryStore) RecordUsage(_ context.Context, record gateway.UsageRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record.ID = int64(len(s.usage) + 1)
	record.CreatedAt = time.Now().UTC()
	s.usage = append(s.usage, record)
	return nil
}

func (s *MemoryStore) ListUsage(_ context.Context, limit int) ([]gateway.UsageRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.usage) {
		limit = len(s.usage)
	}
	records := make([]gateway.UsageRecord, 0, limit)
	for idx := len(s.usage) - 1; idx >= 0 && len(records) < limit; idx-- {
		records = append(records, s.usage[idx])
	}
	return records, nil
}
