package router

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"message-gateway/internal/model"
)

type Router struct {
	logger *slog.Logger
	loader *RulesLoader
}

func New(rulesPath string, sourceURL string, reloadInterval time.Duration, logger *slog.Logger) *Router {
	return &Router{
		logger: logger,
		loader: NewRulesLoader(rulesPath, sourceURL, reloadInterval, logger),
	}
}

func (r *Router) Start(ctx context.Context) {
	r.loader.Start(ctx)
}

func (r *Router) Route(env model.Envelope) model.RouteResult {
	for _, rule := range r.loader.Rules() {
		if rule.Match.MatchEnvelope(env) {
			dedupKey := fmt.Sprintf("%s:%s", env.EventID, rule.ID)
			text := rule.Action.ReplyText
			if strings.TrimSpace(text) == "" {
				text = r.fallback(env).Text
			}
			toast := rule.Action.ToastText
			if toast == "" && env.Kind == model.EnvelopeKindCardAction {
				toast = "已接收，处理中"
			}
			return model.RouteResult{
				Text:      text,
				ToastText: toast,
				DedupKey:  dedupKey,
			}
		}
	}

	return r.fallback(env)
}

func (r *Router) fallback(env model.Envelope) model.RouteResult {
	text := strings.TrimSpace(env.Text)
	if env.Kind == model.EnvelopeKindCardAction {
		if env.ActionName == "" && env.ActionTag == "" {
			return model.RouteResult{
				Text:     "已收到卡片操作。",
				DedupKey: fmt.Sprintf("%s:card_action", env.EventID),
			}
		}
		return model.RouteResult{
			Text:      fmt.Sprintf("已收到卡片操作：%s", firstNonEmpty(env.ActionName, env.ActionTag)),
			DedupKey:  fmt.Sprintf("%s:card_action", env.EventID),
			ToastText: "已接收，处理中",
		}
	}

	if text == "" {
		return model.RouteResult{
			Text:     "已收到消息，但当前只支持文本消息。输入 /help 查看可用命令。",
			DedupKey: fmt.Sprintf("%s:unsupported", env.EventID),
		}
	}

	if strings.HasPrefix(text, "/help") {
		return model.RouteResult{
			Text:     "欢迎使用 message-gateway。\n当前已接入渠道：Lark Bot。\n可用命令：\n- /help 查看帮助\n- 其他任意文本会触发默认回声回复。",
			DedupKey: fmt.Sprintf("%s:help", env.EventID),
		}
	}

	return model.RouteResult{
		Text:     fmt.Sprintf("收到来自 Lark Bot 的消息：%s", text),
		DedupKey: fmt.Sprintf("%s:echo", env.EventID),
	}
}

func firstNonEmpty(items ...string) string {
	for _, it := range items {
		if strings.TrimSpace(it) != "" {
			return it
		}
	}
	return ""
}
