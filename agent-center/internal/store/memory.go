package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"agent-center/internal/agent"
)

type MemoryStore struct {
	mu    sync.RWMutex
	items map[string]agent.RegisteredAgent
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{items: map[string]agent.RegisteredAgent{}}
}

func (s *MemoryStore) Close() {}

func (s *MemoryStore) Migrate(context.Context) error { return nil }

func (s *MemoryStore) UpsertAgent(_ context.Context, item agent.RegisteredAgent) (agent.RegisteredAgent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item = NormalizeAgent(item)
	now := time.Now().UTC()
	existing, ok := s.items[item.Name]
	if ok {
		item.CreatedAt = existing.CreatedAt
		if item.LastSeenAt == nil {
			item.LastSeenAt = existing.LastSeenAt
		}
	} else {
		item.CreatedAt = now
	}
	if item.IsDefault {
		for name, existingItem := range s.items {
			if name == item.Name || !existingItem.IsDefault {
				continue
			}
			existingItem.IsDefault = false
			s.items[name] = existingItem
		}
	}
	item.UpdatedAt = now
	s.items[item.Name] = item
	return item, nil
}

func (s *MemoryStore) ListAgents(_ context.Context, enabledOnly bool) ([]agent.RegisteredAgent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]agent.RegisteredAgent, 0, len(s.items))
	for _, item := range s.items {
		if enabledOnly && !item.Enabled {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].Name < items[j].Name
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) GetAgent(_ context.Context, name string) (agent.RegisteredAgent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normalized := NormalizeAgent(agent.RegisteredAgent{Name: name}).Name
	item, ok := s.items[normalized]
	if !ok {
		return agent.RegisteredAgent{}, ErrNotFound
	}
	return item, nil
}

func (s *MemoryStore) DeleteAgent(_ context.Context, name string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized := NormalizeAgent(agent.RegisteredAgent{Name: name}).Name
	if _, ok := s.items[normalized]; !ok {
		return false, nil
	}
	delete(s.items, normalized)
	return true, nil
}

func (s *MemoryStore) TouchHeartbeat(_ context.Context, name string, now time.Time, status string) (agent.RegisteredAgent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized := NormalizeAgent(agent.RegisteredAgent{Name: name}).Name
	item, ok := s.items[normalized]
	if !ok {
		return agent.RegisteredAgent{}, ErrNotFound
	}
	item.Status = NormalizeHeartbeatStatus(status)
	t := now.UTC()
	item.LastSeenAt = &t
	item.UpdatedAt = t
	s.items[normalized] = item
	return item, nil
}
