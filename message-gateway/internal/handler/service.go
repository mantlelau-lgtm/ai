package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"message-gateway/internal/config"
	"message-gateway/internal/dispatcher"
	"message-gateway/internal/metrics"
	"message-gateway/internal/model"
	"message-gateway/internal/router"
	"message-gateway/internal/store"
)

type Service struct {
	cfg         config.Config
	store       *store.PostgresStore
	router      *router.Router
	lark        *dispatcher.LarkClient
	core        *dispatcher.CoreClient
	botConfig   *dispatcher.BotConfigClient
	agentCenter *dispatcher.AgentCenterClient
	selector    *dispatcher.LLMSelectorClient
	metrics     *metrics.Registry
	logger      *slog.Logger
}

type targetAgentResolution struct {
	Agent      dispatcher.RegisteredAgent
	ResolvedBy string
}

func NewService(
	cfg config.Config,
	store *store.PostgresStore,
	router *router.Router,
	lark *dispatcher.LarkClient,
	core *dispatcher.CoreClient,
	botConfig *dispatcher.BotConfigClient,
	agentCenter *dispatcher.AgentCenterClient,
	selector *dispatcher.LLMSelectorClient,
	metrics *metrics.Registry,
	logger *slog.Logger,
) *Service {
	return &Service{
		cfg:         cfg,
		store:       store,
		router:      router,
		lark:        lark,
		core:        core,
		botConfig:   botConfig,
		agentCenter: agentCenter,
		selector:    selector,
		metrics:     metrics,
		logger:      logger,
	}
}

func (s *Service) HandleInbound(ctx context.Context, env model.Envelope) (model.RouteResult, error) {
	s.logger.Info("inbound event received", "event_id", env.EventID, "message_id", env.MessageID, "bot_id", env.BotID, "chat_id", env.ChatID, "kind", env.Kind)
	s.logger.Info("inbound event payload summary",
		"event_id", env.EventID,
		"bot_id", env.BotID,
		"message_type", env.MessageType,
		"text_preview", strings.TrimSpace(env.Text),
		"sender_open_id", env.SenderOpenID,
	)
	inserted, err := s.store.InsertInboundEvent(ctx, env)
	if err != nil {
		return model.RouteResult{}, err
	}
	if !inserted {
		s.metrics.IncDuplicateEvents()
		return model.RouteResult{}, nil
	}

	s.metrics.IncInboundEvents()

	route := s.router.Route(env)
	s.logger.Info("inbound event routed", "event_id", env.EventID, "dedup_key", route.DedupKey, "has_text", strings.TrimSpace(route.Text) != "", "has_toast", strings.TrimSpace(route.ToastText) != "")

	text := strings.TrimSpace(env.Text)
	if env.Kind == model.EnvelopeKindMessage && strings.HasPrefix(text, "/help") {
		return route, s.enqueueLocalReply(ctx, env, route)
	}

	resolution, resolveErr := s.resolveTargetAgent(ctx, env)
	if resolveErr != nil {
		s.logger.Warn("resolve target agent failed", "event_id", env.EventID, "bot_id", env.BotID, "error", resolveErr)
		return route, s.enqueueUnavailableReply(ctx, env)
	}
	if resolution == nil {
		s.logger.Info("no available agent resolved", "event_id", env.EventID, "bot_id", env.BotID)
		return route, s.enqueueUnavailableReply(ctx, env)
	}

	dedupKey := fmt.Sprintf("%s:forward_to_core", env.EventID)
	if route.DedupKey != "" {
		dedupKey = fmt.Sprintf("%s:%s:forward_to_core", env.EventID, route.DedupKey)
	}
	if err := s.store.EnqueueForwardToCore(ctx, model.ForwardToCorePayload{
		Envelope:        env,
		AgentName:       resolution.Agent.Name,
		AgentRuntimeURL: resolution.Agent.RuntimeURL,
		ResolvedBy:      resolution.ResolvedBy,
	}, dedupKey, s.cfg.WorkerMaxAttempts); err != nil {
		return model.RouteResult{}, err
	}
	s.logger.Info("forward_to_core job enqueued",
		"event_id", env.EventID,
		"bot_id", env.BotID,
		"agent_name", resolution.Agent.Name,
		"agent_runtime_url", resolution.Agent.RuntimeURL,
		"resolved_by", resolution.ResolvedBy,
		"dedup_key", dedupKey,
	)
	s.metrics.IncJobsCreated()
	return route, nil
}

func (s *Service) enqueueLocalReply(ctx context.Context, env model.Envelope, route model.RouteResult) error {
	return s.enqueueTextReply(ctx, env, route.Text, route.DedupKey)
}

func (s *Service) enqueueUnavailableReply(ctx context.Context, env model.Envelope) error {
	err := s.enqueueTextReply(ctx, env, s.cfg.AgentUnavailableReplyText, fmt.Sprintf("%s:agent_unavailable", env.EventID))
	if err != nil {
		s.logger.Warn("enqueue unavailable reply failed", "event_id", env.EventID, "bot_id", env.BotID, "error", err)
		return err
	}
	s.logger.Info("unavailable reply enqueued", "event_id", env.EventID, "bot_id", env.BotID, "reply_text", s.cfg.AgentUnavailableReplyText)
	return nil
}

func (s *Service) enqueueTextReply(ctx context.Context, env model.Envelope, text string, dedupKey string) error {
	content, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}

	botID := strings.TrimSpace(env.BotID)
	if botID == "" {
		botID = strings.TrimSpace(s.cfg.LarkAppID)
	}

	receiveID := env.ChatID
	receiveIDType := "chat_id"
	if receiveID == "" {
		receiveID = env.SenderOpenID
		receiveIDType = "open_id"
	}

	err = s.store.EnqueueSendMessage(ctx, model.SendMessagePayload{
		BotID:         botID,
		ReceiveID:     receiveID,
		ReceiveIDType: receiveIDType,
		MsgType:       "text",
		Content:       string(content),
		UUID:          dedupKey,
	}, dedupKey, s.cfg.WorkerMaxAttempts)
	if err != nil {
		return err
	}

	s.metrics.IncJobsCreated()
	return nil
}

func (s *Service) ProcessPendingJobs(ctx context.Context) error {
	jobs, err := s.store.ClaimPendingJobs(ctx, s.cfg.WorkerBatchSize)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		if err := s.processJob(ctx, job); err != nil {
			s.logger.Error("process job failed", "job_id", job.ID, "error", err)
		}
	}

	return nil
}

func (s *Service) processJob(ctx context.Context, job model.Job) error {
	s.logger.Info("job processing started", "job_id", job.ID, "job_type", job.JobType, "attempts", job.Attempts, "max_attempts", job.MaxAttempts)
	switch job.JobType {
	case "send_message":
		return s.processSendMessageJob(ctx, job)
	case "forward_to_core":
		return s.processForwardToCoreJob(ctx, job)
	default:
		return s.store.MarkJobDead(ctx, job.ID, "unsupported job type")
	}
}

func (s *Service) processSendMessageJob(ctx context.Context, job model.Job) error {
	var payload model.SendMessagePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		s.metrics.IncJobsDead()
		return s.store.MarkJobDead(ctx, job.ID, fmt.Sprintf("invalid payload: %v", err))
	}

	if err := s.lark.SendMessage(ctx, strings.TrimSpace(payload.BotID), payload); err != nil {
		s.logger.Warn("send message failed",
			"job_id", job.ID,
			"bot_id", strings.TrimSpace(payload.BotID),
			"receive_id", payload.ReceiveID,
			"receive_id_type", payload.ReceiveIDType,
			"attempts", job.Attempts,
			"error", err,
		)
		if job.Attempts+1 >= job.MaxAttempts {
			s.metrics.IncJobsDead()
			return s.store.MarkJobDead(ctx, job.ID, err.Error())
		}

		nextRunAt := time.Now().Add(s.cfg.WorkerRetryBaseInterval * time.Duration(1<<job.Attempts))
		s.metrics.IncJobsRetried()
		return s.store.MarkJobRetry(ctx, job.ID, err.Error(), nextRunAt)
	}

	s.metrics.IncOutboundSent()
	s.metrics.IncJobsSucceeded()
	return s.store.MarkJobSucceeded(ctx, job.ID)
}

func (s *Service) processForwardToCoreText(ctx context.Context, job model.Job, env model.Envelope, botID string, sessionID string) error {
	var payload model.ForwardToCorePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		s.metrics.IncJobsDead()
		return s.store.MarkJobDead(ctx, job.ID, fmt.Sprintf("invalid payload: %v", err))
	}
	res, err := s.core.StreamReplyToBaseURL(ctx, payload.AgentRuntimeURL, env, botID, sessionID)
	if err != nil {
		if job.Attempts+1 >= job.MaxAttempts {
			s.metrics.IncJobsDead()
			return s.store.MarkJobDead(ctx, job.ID, err.Error())
		}
		nextRunAt := time.Now().Add(s.cfg.WorkerRetryBaseInterval * time.Duration(1<<job.Attempts))
		s.metrics.IncJobsRetried()
		return s.store.MarkJobRetry(ctx, job.ID, err.Error(), nextRunAt)
	}

	if strings.TrimSpace(res.Text) != "" {
		receiveID := env.ChatID
		receiveIDType := "chat_id"
		if receiveID == "" {
			receiveID = env.SenderOpenID
			receiveIDType = "open_id"
		}

		dedupKey := fmt.Sprintf("%s:%s:core_reply", env.EventID, strings.TrimSpace(payload.AgentName))
		err = s.store.EnqueueSendMessage(ctx, model.SendMessagePayload{
			BotID:         botID,
			ReceiveID:     receiveID,
			ReceiveIDType: receiveIDType,
			MsgType:       "text",
			Content:       textMessageContent(res.Text),
			UUID:          dedupKey,
		}, dedupKey, s.cfg.WorkerMaxAttempts)
		if err != nil {
			if job.Attempts+1 >= job.MaxAttempts {
				s.metrics.IncJobsDead()
				return s.store.MarkJobDead(ctx, job.ID, err.Error())
			}
			nextRunAt := time.Now().Add(s.cfg.WorkerRetryBaseInterval * time.Duration(1<<job.Attempts))
			s.metrics.IncJobsRetried()
			return s.store.MarkJobRetry(ctx, job.ID, err.Error(), nextRunAt)
		}

		s.metrics.IncJobsCreated()
	}

	s.logger.Info("forward_to_core job completed", "job_id", job.ID, "event_id", env.EventID, "agent_name", strings.TrimSpace(payload.AgentName), "reply_chars", len(res.Text))
	s.metrics.IncJobsSucceeded()
	return s.store.MarkJobSucceeded(ctx, job.ID)
}

func (s *Service) processForwardToCoreJob(ctx context.Context, job model.Job) error {
	if s.core == nil {
		s.metrics.IncJobsDead()
		return s.store.MarkJobDead(ctx, job.ID, "core not configured")
	}

	var payload model.ForwardToCorePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		s.metrics.IncJobsDead()
		return s.store.MarkJobDead(ctx, job.ID, fmt.Sprintf("invalid payload: %v", err))
	}
	if strings.TrimSpace(payload.AgentRuntimeURL) == "" {
		s.metrics.IncJobsDead()
		return s.store.MarkJobDead(ctx, job.ID, "agent runtime url is empty")
	}

	env := payload.Envelope
	botID := strings.TrimSpace(env.BotID)
	if botID == "" {
		botID = strings.TrimSpace(s.cfg.LarkAppID)
	}
	sessionID := env.ChatID
	if sessionID == "" {
		sessionID = env.SenderUserID
	}
	if sessionID == "" {
		sessionID = env.SenderOpenID
	}
	if sessionID == "" {
		sessionID = env.EventID
	}

	s.logger.Info("forward_to_core job started", "job_id", job.ID, "event_id", env.EventID, "bot_id", botID, "session_id", sessionID, "agent_name", payload.AgentName, "agent_runtime_url", payload.AgentRuntimeURL, "resolved_by", payload.ResolvedBy, "streaming_card", s.cfg.LarkStreamingCardEnabled)
	return s.processForwardToCoreText(ctx, job, env, botID, sessionID)

	receiveID := env.ChatID
	receiveIDType := "chat_id"
	if receiveID == "" {
		receiveID = env.SenderOpenID
		receiveIDType = "open_id"
	}

	title := strings.TrimSpace(env.Text)
	if title == "" {
		title = "AI 回复"
	}
	if i := strings.IndexByte(title, '\n'); i > 0 {
		title = strings.TrimSpace(title[:i])
	}
	if len(title) > 60 {
		title = strings.TrimSpace(title[:60])
	}

	messageID, err := s.lark.CreateMessage(ctx, botID, model.SendMessagePayload{
		BotID:         botID,
		ReceiveID:     receiveID,
		ReceiveIDType: receiveIDType,
		MsgType:       "text",
		Content:       textMessageContent("生成中..."),
		UUID:          streamingMessageUUID(env.EventID),
	})
	if err != nil {
		if job.Attempts+1 >= job.MaxAttempts {
			s.metrics.IncJobsDead()
			return s.store.MarkJobDead(ctx, job.ID, err.Error())
		}
		nextRunAt := time.Now().Add(s.cfg.WorkerRetryBaseInterval * time.Duration(1<<job.Attempts))
		s.metrics.IncJobsRetried()
		return s.store.MarkJobRetry(ctx, job.ID, err.Error(), nextRunAt)
	}
	s.metrics.IncOutboundSent()

	var full strings.Builder
	lastUpdateAt := time.Now()

	updateStreaming := func() error {
		display := limitCardText(full.String(), s.cfg.LarkStreamingCardMaxBytes)
		if strings.TrimSpace(display) == "" {
			display = "生成中..."
		}
		return s.lark.PatchMessage(ctx, botID, messageID, textMessageContent(display))
	}

	updateFinal := func() error {
		display := limitCardText(full.String(), s.cfg.LarkStreamingCardMaxBytes)
		if strings.TrimSpace(display) == "" {
			display = "无回复内容"
		}
		return s.lark.PatchMessage(ctx, botID, messageID, textMessageContent(display))
	}

	err = s.core.StreamReplyChunks(ctx, env, botID, sessionID, func(delta string) error {
		if delta != "" {
			full.WriteString(delta)
		}
		if time.Since(lastUpdateAt) < s.cfg.LarkStreamingCardUpdate {
			return nil
		}
		lastUpdateAt = time.Now()
		return updateStreaming()
	})
	if err != nil {
		if job.Attempts+1 >= job.MaxAttempts {
			s.metrics.IncJobsDead()
			return s.store.MarkJobDead(ctx, job.ID, err.Error())
		}
		nextRunAt := time.Now().Add(s.cfg.WorkerRetryBaseInterval * time.Duration(1<<job.Attempts))
		s.metrics.IncJobsRetried()
		return s.store.MarkJobRetry(ctx, job.ID, err.Error(), nextRunAt)
	}

	if err := updateFinal(); err != nil {
		if job.Attempts+1 >= job.MaxAttempts {
			s.metrics.IncJobsDead()
			return s.store.MarkJobDead(ctx, job.ID, err.Error())
		}
		nextRunAt := time.Now().Add(s.cfg.WorkerRetryBaseInterval * time.Duration(1<<job.Attempts))
		s.metrics.IncJobsRetried()
		return s.store.MarkJobRetry(ctx, job.ID, err.Error(), nextRunAt)
	}

	s.logger.Info("forward_to_core streaming job completed", "job_id", job.ID, "event_id", env.EventID, "message_id", messageID, "reply_chars", full.Len())
	s.metrics.IncJobsSucceeded()
	return s.store.MarkJobSucceeded(ctx, job.ID)
}

func (s *Service) resolveTargetAgent(ctx context.Context, env model.Envelope) (*targetAgentResolution, error) {
	if s.agentCenter == nil {
		return nil, fmt.Errorf("agent center not configured")
	}

	botID := strings.TrimSpace(env.BotID)
	if botID == "" {
		botID = strings.TrimSpace(s.cfg.LarkAppID)
	}
	env.BotID = botID

	agents, err := s.agentCenter.ListAvailableAgents(ctx)
	if err != nil {
		return nil, err
	}
	if len(agents) == 0 {
		return nil, nil
	}
	defaultAgent, hasDefault := findDefaultAgent(agents)

	botAgentName := ""
	if s.botConfig != nil && botID != "" {
		if botCfg, botErr := s.botConfig.GetBotConfig(ctx, botID); botErr != nil {
			s.logger.Warn("load bot config failed", "bot_id", botID, "error", botErr)
		} else {
			botAgentName = strings.ToLower(strings.TrimSpace(botCfg.AgentName))
		}
	}

	if botAgentName != "" {
		if matched, ok := findAgentByName(agents, botAgentName); ok {
			return &targetAgentResolution{Agent: matched, ResolvedBy: "bot_binding"}, nil
		}
		if hasDefault {
			s.logger.Info("fallback to default agent after bot binding miss", "event_id", env.EventID, "bot_id", botID, "requested_agent", botAgentName, "default_agent", defaultAgent.Name)
			return &targetAgentResolution{Agent: defaultAgent, ResolvedBy: "default_fallback"}, nil
		}
		return nil, nil
	}

	routableAgents := filterRoutableAgents(agents)
	if len(routableAgents) == 0 {
		if hasDefault {
			return &targetAgentResolution{Agent: defaultAgent, ResolvedBy: "default_fallback"}, nil
		}
		return nil, nil
	}

	if s.selector == nil {
		if hasDefault {
			return &targetAgentResolution{Agent: defaultAgent, ResolvedBy: "default_fallback"}, nil
		}
		return nil, nil
	}

	selectedName, reason, err := s.selector.SelectAgent(ctx, env, routableAgents)
	if err != nil {
		return nil, err
	}
	if selectedName == "" {
		if hasDefault {
			return &targetAgentResolution{Agent: defaultAgent, ResolvedBy: "default_fallback"}, nil
		}
		return nil, nil
	}
	matched, ok := findAgentByName(routableAgents, selectedName)
	if !ok {
		s.logger.Warn("selected agent not found in available list", "selected_agent", selectedName, "reason", reason)
		if hasDefault {
			return &targetAgentResolution{Agent: defaultAgent, ResolvedBy: "default_fallback"}, nil
		}
		return nil, nil
	}
	s.logger.Info("agent selected by llm", "event_id", env.EventID, "bot_id", botID, "selected_agent", matched.Name, "reason", reason)
	return &targetAgentResolution{Agent: matched, ResolvedBy: "llm"}, nil
}

func findAgentByName(agents []dispatcher.RegisteredAgent, name string) (dispatcher.RegisteredAgent, bool) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	for _, item := range agents {
		if strings.ToLower(strings.TrimSpace(item.Name)) == normalized {
			return item, true
		}
	}
	return dispatcher.RegisteredAgent{}, false
}

func findDefaultAgent(agents []dispatcher.RegisteredAgent) (dispatcher.RegisteredAgent, bool) {
	for _, item := range agents {
		if item.IsDefault {
			return item, true
		}
	}
	return dispatcher.RegisteredAgent{}, false
}

func filterRoutableAgents(agents []dispatcher.RegisteredAgent) []dispatcher.RegisteredAgent {
	items := make([]dispatcher.RegisteredAgent, 0, len(agents))
	for _, item := range agents {
		if item.IsDefault {
			continue
		}
		items = append(items, item)
	}
	return items
}

func textMessageContent(text string) string {
	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return `{"text":""}`
	}
	return string(payload)
}

func streamingMessageUUID(eventID string) string {
	id := strings.TrimSpace(eventID)
	if len(id) > 24 {
		id = id[:24]
	}
	if id == "" {
		id = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return "mgw:" + id
}

func limitCardText(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	prefix := "...(内容过长已截断，仅展示末尾)\n\n"
	remain := maxBytes - len(prefix)
	if remain <= 0 {
		return prefix
	}
	if len(s) <= remain {
		return prefix + s
	}
	start := len(s) - remain
	for start < len(s) && start > 0 && (s[start]&0xC0) == 0x80 {
		start++
	}
	tail := s[start:]
	if !utf8.ValidString(tail) {
		r := []rune(tail)
		tail = string(r)
	}
	return prefix + tail
}
