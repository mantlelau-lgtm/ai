package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"agent-center/internal/agent"
)

var ErrNotFound = errors.New("agent not found")

type Store interface {
	Migrate(ctx context.Context) error
	UpsertAgent(ctx context.Context, item agent.RegisteredAgent) (agent.RegisteredAgent, error)
	ListAgents(ctx context.Context, enabledOnly bool) ([]agent.RegisteredAgent, error)
	GetAgent(ctx context.Context, name string) (agent.RegisteredAgent, error)
	DeleteAgent(ctx context.Context, name string) (bool, error)
	TouchHeartbeat(ctx context.Context, name string, now time.Time, status string) (agent.RegisteredAgent, error)
	Close()
}

func NormalizeAgent(item agent.RegisteredAgent) agent.RegisteredAgent {
	item.Name = strings.ToLower(strings.TrimSpace(item.Name))
	item.Type = strings.TrimSpace(item.Type)
	if item.Type == "" {
		item.Type = "custom"
	}
	item.Source = strings.TrimSpace(item.Source)
	if item.Source == "" {
		item.Source = "local"
	}
	item.Description = strings.TrimSpace(item.Description)
	item.KeyName = strings.TrimSpace(item.KeyName)
	item.RuntimeURL = strings.TrimSpace(item.RuntimeURL)
	item.WorkspacePath = strings.TrimSpace(item.WorkspacePath)
	item.Entrypoint = strings.TrimSpace(item.Entrypoint)
	item.Owner = strings.TrimSpace(item.Owner)
	item.Status = strings.ToLower(strings.TrimSpace(item.Status))
	if item.Status == "" {
		item.Status = "registered"
	}
	item.Tools = uniqueTrimmed(item.Tools)
	item.Tags = uniqueTrimmed(item.Tags)
	if item.Metadata == nil {
		item.Metadata = map[string]string{}
	}
	if item.Name == "" {
		return item
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Time{}
	}
	return item
}

func ClearDefaultAgent(items []agent.RegisteredAgent, exceptName string) []agent.RegisteredAgent {
	normalizedExcept := strings.ToLower(strings.TrimSpace(exceptName))
	out := make([]agent.RegisteredAgent, 0, len(items))
	for _, item := range items {
		if !item.IsDefault {
			out = append(out, item)
			continue
		}
		if strings.ToLower(strings.TrimSpace(item.Name)) == normalizedExcept {
			out = append(out, item)
			continue
		}
		item.IsDefault = false
		out = append(out, item)
	}
	return out
}

func uniqueTrimmed(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if out == nil {
		return []string{}
	}
	return out
}

func NormalizeHeartbeatStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" {
		return "online"
	}
	return normalized
}

func ProjectAgent(item agent.RegisteredAgent, now time.Time, offlineTimeout time.Duration) agent.RegisteredAgent {
	item = NormalizeAgent(item)
	if !item.Enabled {
		item.Status = "disabled"
		return item
	}

	switch item.Status {
	case "disabled", "unavailable", "offline":
		return item
	}

	if item.LastSeenAt == nil {
		if item.Status == "online" {
			item.Status = "offline"
		}
		return item
	}

	lastSeenAt := item.LastSeenAt.UTC()
	item.LastSeenAt = &lastSeenAt
	if offlineTimeout > 0 && now.UTC().After(lastSeenAt.Add(offlineTimeout)) {
		item.Status = "offline"
		return item
	}
	if item.Status == "registered" {
		item.Status = "online"
	}
	return item
}

func ProjectAgents(items []agent.RegisteredAgent, now time.Time, offlineTimeout time.Duration) []agent.RegisteredAgent {
	projected := make([]agent.RegisteredAgent, 0, len(items))
	for _, item := range items {
		projected = append(projected, ProjectAgent(item, now, offlineTimeout))
	}
	return projected
}

func IsRuntimeAvailable(item agent.RegisteredAgent) bool {
	return item.Enabled && item.RuntimeURL != "" && item.Status == "online"
}
