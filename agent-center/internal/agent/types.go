package agent

import "time"

type RegisteredAgent struct {
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Source        string            `json:"source"`
	Description   string            `json:"description,omitempty"`
	KeyName       string            `json:"key_name,omitempty"`
	IsDefault     bool              `json:"is_default,omitempty"`
	Tools         []string          `json:"tools,omitempty"`
	RuntimeURL    string            `json:"runtime_url,omitempty"`
	WorkspacePath string            `json:"workspace_path,omitempty"`
	Entrypoint    string            `json:"entrypoint,omitempty"`
	Owner         string            `json:"owner,omitempty"`
	Tags          []string          `json:"tags,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Enabled       bool              `json:"enabled"`
	Status        string            `json:"status,omitempty"`
	LastSeenAt    *time.Time        `json:"last_seen_at,omitempty"`
	CreatedAt     time.Time         `json:"created_at,omitempty"`
	UpdatedAt     time.Time         `json:"updated_at,omitempty"`
}

type RegisterAgentsRequest struct {
	Agents []RegisteredAgent `json:"agents"`
}

type RegisterAgentRequest struct {
	Agent RegisteredAgent `json:"agent"`
}

type HeartbeatRequest struct {
	Status string `json:"status,omitempty"`
}
