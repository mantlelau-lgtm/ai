package app

import (
	"context"
	"log/slog"
	"net/http"
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
	rt := router.New(cfg.RouteRulesPath, cfg.RouteRulesReloadInterval, logger)
	rt.Start(ctx)

	larkClient := mgwdispatcher.NewLarkClient(cfg)
	coreClient := mgwdispatcher.NewCoreClient(cfg)
	svc := handler.NewService(cfg, st, rt, larkClient, coreClient, m, logger)

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
		wsClient := larkws.NewClient(
			cfg.LarkAppID,
			cfg.LarkAppSecret,
			larkws.WithEventHandler(eventDispatcher),
			larkws.WithDomain(cfg.LarkOpenBaseURL),
			larkws.WithLogLevel(larkcore.LogLevelInfo),
		)

		go func() {
			if err := wsClient.Start(ctx); err != nil {
				logger.Error("lark ws client start failed", "error", err)
			}
		}()
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
			pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
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
