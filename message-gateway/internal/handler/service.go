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

	if !s.cfg.LarkStreamingCardEnabled {
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
			content, err := json.Marshal(map[string]string{"text": res.Text})
			if err != nil {
				if job.Attempts+1 >= job.MaxAttempts {
					s.metrics.IncJobsDead()
					return s.store.MarkJobDead(ctx, job.ID, err.Error())
				}
				nextRunAt := time.Now().Add(s.cfg.WorkerRetryBaseInterval * time.Duration(1<<job.Attempts))
				s.metrics.IncJobsRetried()
				return s.store.MarkJobRetry(ctx, job.ID, err.Error(), nextRunAt)
			}

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
				Content:       string(content),
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

		s.metrics.IncJobsSucceeded()
		return s.store.MarkJobSucceeded(ctx, job.ID)
	}

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

	cardJSON, err := dispatcher.BuildStreamingCard(fmt.Sprintf("# %s\n\n", title))
	if err != nil {
		if job.Attempts+1 >= job.MaxAttempts {
			s.metrics.IncJobsDead()
			return s.store.MarkJobDead(ctx, job.ID, err.Error())
		}
		nextRunAt := time.Now().Add(s.cfg.WorkerRetryBaseInterval * time.Duration(1<<job.Attempts))
		s.metrics.IncJobsRetried()
		return s.store.MarkJobRetry(ctx, job.ID, err.Error(), nextRunAt)
	}

	messageID, err := s.lark.CreateMessage(ctx, botID, model.SendMessagePayload{
		BotID:         botID,
		ReceiveID:     receiveID,
		ReceiveIDType: receiveIDType,
		MsgType:       "interactive",
		Content:       cardJSON,
		UUID:          fmt.Sprintf("mgw:%s:core_stream:%s", env.EventID, job.ID),
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
		c, err := dispatcher.BuildStreamingCard(fmt.Sprintf("# %s\n\n%s", title, display))
		if err != nil {
			return err
		}
		return s.lark.PatchMessage(ctx, botID, messageID, c)
	}

	updateFinal := func() error {
		display := limitCardText(full.String(), s.cfg.LarkStreamingCardMaxBytes)
		c, err := dispatcher.BuildFinalCard(fmt.Sprintf("# %s\n\n%s", title, display))
		if err != nil {
			return err
		}
		return s.lark.PatchMessage(ctx, botID, messageID, c)
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

	s.metrics.IncJobsSucceeded()
	return s.store.MarkJobSucceeded(ctx, job.ID)
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
