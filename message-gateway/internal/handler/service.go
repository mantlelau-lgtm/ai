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
	cfg     config.Config
	store   *store.PostgresStore
	router  *router.Router
	lark    *dispatcher.LarkClient
	core    *dispatcher.CoreClient
	metrics *metrics.Registry
	logger  *slog.Logger
}

func NewService(
	cfg config.Config,
	store *store.PostgresStore,
	router *router.Router,
	lark *dispatcher.LarkClient,
	core *dispatcher.CoreClient,
	metrics *metrics.Registry,
	logger *slog.Logger,
) *Service {
	return &Service{
		cfg:     cfg,
		store:   store,
		router:  router,
		lark:    lark,
		core:    core,
		metrics: metrics,
		logger:  logger,
	}
}

func (s *Service) HandleInbound(ctx context.Context, env model.Envelope) (model.RouteResult, error) {
	s.logger.Info("inbound event received", "event_id", env.EventID, "message_id", env.MessageID, "bot_id", env.BotID, "chat_id", env.ChatID, "kind", env.Kind)
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

	if s.core != nil {
		dedupKey := fmt.Sprintf("%s:forward_to_core", env.EventID)
		if route.DedupKey != "" {
			dedupKey = fmt.Sprintf("%s:%s:forward_to_core", env.EventID, route.DedupKey)
		}
		if err := s.store.EnqueueForwardToCore(ctx, model.ForwardToCorePayload{Envelope: env}, dedupKey, s.cfg.WorkerMaxAttempts); err != nil {
			return model.RouteResult{}, err
		}
		s.logger.Info("forward_to_core job enqueued", "event_id", env.EventID, "bot_id", env.BotID, "dedup_key", dedupKey)
		s.metrics.IncJobsCreated()
		return route, nil
	}

	return route, s.enqueueLocalReply(ctx, env, route)
}

func (s *Service) enqueueLocalReply(ctx context.Context, env model.Envelope, route model.RouteResult) error {
	content, err := json.Marshal(map[string]string{"text": route.Text})
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
		UUID:          route.DedupKey,
	}, route.DedupKey, s.cfg.WorkerMaxAttempts)
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
	res, err := s.core.StreamReply(ctx, env, botID, sessionID)
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

		dedupKey := fmt.Sprintf("%s:core_reply", env.EventID)
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

	s.logger.Info("forward_to_core job completed", "job_id", job.ID, "event_id", env.EventID, "reply_chars", len(res.Text))
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

	s.logger.Info("forward_to_core job started", "job_id", job.ID, "event_id", env.EventID, "bot_id", botID, "session_id", sessionID, "streaming_card", s.cfg.LarkStreamingCardEnabled)
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
