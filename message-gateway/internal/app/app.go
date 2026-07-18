package app

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"message-gateway/internal/config"
	mgwdispatcher "message-gateway/internal/dispatcher"
	"message-gateway/internal/handler"
	"message-gateway/internal/ingress"
	"message-gateway/internal/metrics"
	"message-gateway/internal/router"
	"message-gateway/internal/store"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/core/httpserverext"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	larkdispatcher "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

type App struct {
	cfg     config.Config
	store   *store.PostgresStore
	service *handler.Service
	http    *ingress.HTTPHandler
	logger  *slog.Logger
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	st, err := store.NewPostgresStore(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if err := st.Migrate(ctx); err != nil {
		st.Close()
		return nil, err
	}

	m := metrics.New()
	rulesURL := ""
	if cfg.AdminConfigBaseURL != "" {
		rulesURL = cfg.AdminConfigBaseURL + cfg.AdminMessageRoutesPath
	}
	rt := router.New(cfg.RouteRulesPath, rulesURL, cfg.RouteRulesReloadInterval, logger)
	rt.Start(ctx)

	larkClient := mgwdispatcher.NewLarkClient(cfg)
	coreClient := mgwdispatcher.NewCoreClient(cfg)
	botConfigClient := mgwdispatcher.NewBotConfigClient(cfg)
	agentCenterClient := mgwdispatcher.NewAgentCenterClient(cfg)
	llmSelectorClient := mgwdispatcher.NewLLMSelectorClient(cfg)
	svc := handler.NewService(cfg, st, rt, larkClient, coreClient, botConfigClient, agentCenterClient, llmSelectorClient, m, logger)

	eventDispatcher := larkdispatcher.NewEventDispatcher(cfg.LarkVerificationToken, cfg.LarkEncryptKey)
	eventDispatcher.OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
		env, err := envelopeFromP2MessageReceiveV1(event)
		if err != nil {
			logger.Error("normalize p2 message failed", "error", err)
			return nil
		}
		_, err = svc.HandleInbound(ctx, env)
		return err
	}).OnP1MessageReceiveV1(func(ctx context.Context, event *larkim.P1MessageReceiveV1) error {
		env, err := envelopeFromP1MessageReceiveV1(event)
		if err != nil {
			logger.Error("normalize p1 message failed", "error", err)
			return nil
		}
		_, err = svc.HandleInbound(ctx, env)
		return err
	}).OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
		env, err := envelopeFromP2CardActionTrigger(event)
		if err != nil {
			logger.Error("normalize p2 card action failed", "error", err)
			return nil, nil
		}
		route, err := svc.HandleInbound(ctx, env)
		if err != nil {
			return nil, err
		}
		if route.ToastText == "" {
			return nil, nil
		}
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{
				Type:    "info",
				Content: route.ToastText,
			},
		}, nil
	})

	if cfg.LarkWSEnabled {
		startLarkWSClients(ctx, cfg, eventDispatcher, logger)
	}

	callback := httpserverext.NewEventHandlerFunc(eventDispatcher, larkevent.WithLogLevel(larkcore.LogLevelInfo))
	httpHandler := ingress.NewHTTPHandler(st, m, callback, logger)

	return &App{
		cfg:     cfg,
		store:   st,
		service: svc,
		http:    httpHandler,
		logger:  logger,
	}, nil
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	a.http.Register(mux)
	return mux
}

func (a *App) RunWorker(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.WorkerPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pollCtx, cancel := context.WithTimeout(ctx, a.cfg.WorkerJobTimeout)
			if reset, err := a.store.ResetStaleRunningJobs(pollCtx, a.cfg.WorkerJobTimeout); err != nil {
				a.logger.Error("reset stale running jobs failed", "error", err)
			} else if reset > 0 {
				a.logger.Warn("stale running jobs reset", "count", reset)
			}
			if err := a.service.ProcessPendingJobs(pollCtx); err != nil {
				a.logger.Error("worker poll failed", "error", err)
			}
			cancel()
		}
	}
}

func (a *App) Close() {
	a.store.Close()
}

func startLarkWSClients(ctx context.Context, cfg config.Config, eventDispatcher *larkdispatcher.EventDispatcher, logger *slog.Logger) {
	var mu sync.Mutex
	started := map[string]struct{}{}

	launch := func(appID string, wsClient *larkws.Client) {
		go func() {
			logger.Info("starting lark ws client", "app_id", appID)
			if err := wsClient.Start(ctx); err != nil && ctx.Err() == nil {
				logger.Error("lark ws client start failed", "app_id", appID, "error", err)
			}
			mu.Lock()
			delete(started, appID)
			mu.Unlock()
		}()
	}

	loadAndStart := func() {
		bots := mgwdispatcher.LoadLarkBotCredentials(cfg)
		if len(bots) == 0 && cfg.LarkAppID != "" && cfg.LarkAppSecret != "" {
			bots = []mgwdispatcher.LarkBotCredential{{
				BotID:       cfg.LarkAppID,
				AppID:       cfg.LarkAppID,
				AppSecret:   cfg.LarkAppSecret,
				OpenBaseURL: cfg.LarkOpenBaseURL,
			}}
		}
		if len(bots) == 0 {
			logger.Warn("no lark ws credentials available yet")
			return
		}

		for _, b := range bots {
			appID := strings.TrimSpace(b.AppID)
			if appID == "" {
				appID = strings.TrimSpace(b.BotID)
			}
			appSecret := strings.TrimSpace(b.AppSecret)
			if appID == "" || appSecret == "" {
				continue
			}

			mu.Lock()
			if _, ok := started[appID]; ok {
				mu.Unlock()
				continue
			}
			started[appID] = struct{}{}
			mu.Unlock()

			domain := strings.TrimSpace(b.OpenBaseURL)
			if domain == "" {
				domain = cfg.LarkOpenBaseURL
			}
			wsClient := larkws.NewClient(
				appID,
				appSecret,
				larkws.WithEventHandler(eventDispatcher),
				larkws.WithDomain(domain),
				larkws.WithLogLevel(larkcore.LogLevelInfo),
			)
			launch(appID, wsClient)
		}
	}

	go func() {
		loadAndStart()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				loadAndStart()
			}
		}
	}()
}
