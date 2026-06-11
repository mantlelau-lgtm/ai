package router

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"message-gateway/internal/model"
)

type RuleSet struct {
	Rules []Rule `json:"rules"`
}

type Rule struct {
	ID       string     `json:"id"`
	Priority int        `json:"priority"`
	Match    RuleMatch  `json:"match"`
	Action   RuleAction `json:"action"`
}

type RuleMatch struct {
	Kind       string `json:"kind,omitempty"`
	EventType  string `json:"event_type,omitempty"`
	TextEquals string `json:"text_equals,omitempty"`
	TextPrefix string `json:"text_prefix,omitempty"`
	ActionName string `json:"action_name,omitempty"`
	ActionTag  string `json:"action_tag,omitempty"`
}

type RuleAction struct {
	ReplyText string `json:"reply_text,omitempty"`
	ToastText string `json:"toast_text,omitempty"`
}

type ruleState struct {
	rules   []Rule
	modTime time.Time
}

type RulesLoader struct {
	path           string
	reloadInterval time.Duration
	logger         *slog.Logger
	state          atomic.Value
}

func NewRulesLoader(path string, reloadInterval time.Duration, logger *slog.Logger) *RulesLoader {
	rl := &RulesLoader{
		path:           path,
		reloadInterval: reloadInterval,
		logger:         logger,
	}
	rl.state.Store(ruleState{})
	return rl
}

func (r *RulesLoader) Start(ctx context.Context) {
	if r.path == "" || r.reloadInterval <= 0 {
		return
	}

	r.tryReload(ctx, true)

	ticker := time.NewTicker(r.reloadInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.tryReload(ctx, false)
			}
		}
	}()
}

func (r *RulesLoader) Rules() []Rule {
	st := r.state.Load().(ruleState)
	return st.rules
}

func (r *RulesLoader) tryReload(ctx context.Context, force bool) {
	info, err := os.Stat(r.path)
	if err != nil {
		if force {
			r.logger.Warn("route rules stat failed", "path", r.path, "error", err)
		}
		return
	}

	st := r.state.Load().(ruleState)
	if !force && !info.ModTime().After(st.modTime) {
		return
	}

	b, err := os.ReadFile(r.path)
	if err != nil {
		r.logger.Warn("route rules read failed", "path", r.path, "error", err)
		return
	}

	rules, err := parseRules(b)
	if err != nil {
		r.logger.Warn("route rules parse failed", "path", r.path, "error", err)
		return
	}

	r.state.Store(ruleState{rules: rules, modTime: info.ModTime()})
	r.logger.Info("route rules loaded", "path", r.path, "count", len(rules), "mtime", info.ModTime())
}

func parseRules(b []byte) ([]Rule, error) {
	var rs RuleSet
	if err := json.Unmarshal(b, &rs); err == nil && len(rs.Rules) > 0 {
		return normalizeRules(rs.Rules), nil
	}

	var rules []Rule
	if err := json.Unmarshal(b, &rules); err != nil {
		return nil, err
	}
	return normalizeRules(rules), nil
}

func normalizeRules(rules []Rule) []Rule {
	out := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if strings.TrimSpace(r.ID) == "" {
			continue
		}
		out = append(out, r)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Priority > out[j].Priority
	})

	return out
}

func (m RuleMatch) MatchEnvelope(env model.Envelope) bool {
	if m.Kind != "" && env.Kind != m.Kind {
		return false
	}
	if m.EventType != "" && env.EventType != m.EventType {
		return false
	}
	if m.TextEquals != "" && env.Text != m.TextEquals {
		return false
	}
	if m.TextPrefix != "" && !strings.HasPrefix(env.Text, m.TextPrefix) {
		return false
	}
	if m.ActionName != "" && env.ActionName != m.ActionName {
		return false
	}
	if m.ActionTag != "" && env.ActionTag != m.ActionTag {
		return false
	}
	return true
}
