package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"llm-gateway/internal/config"
	"llm-gateway/internal/httpapi"
	"llm-gateway/internal/logging"
	"llm-gateway/internal/provider"
	"llm-gateway/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logFile, logOutput, err := logging.MultiOutput("logs", os.Stdout)
	if err != nil {
		log.Fatalf("init hourly log writer: %v", err)
	}
	defer logFile.Close()
	logger := slog.New(slog.NewJSONHandler(logOutput, nil))
	slog.SetDefault(logger)

	pgStore, err := store.NewPostgresStore(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("init postgres store: %v", err)
	}
	defer pgStore.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := pgStore.Migrate(ctx); err != nil {
		log.Fatalf("migrate store: %v", err)
	}

	manager := provider.NewManager(pgStore)
	reloadCatalog := func(ctx context.Context) error {
		if cfg.AdminConfigBaseURL != "" {
			models, err := config.ApplyCatalogFromURL(ctx, pgStore, cfg.AdminConfigBaseURL+cfg.AdminCatalogPath)
			if err == nil {
				manager.SetModelRoutes(models)
				return nil
			}
			if cfg.CatalogPath == "" {
				return err
			}
		}
		if cfg.CatalogPath == "" {
			manager.SetModelRoutes(nil)
			return nil
		}
		models, err := config.ApplyCatalog(ctx, pgStore, cfg.CatalogPath)
		if err != nil {
			return err
		}
		manager.SetModelRoutes(models)
		return nil
	}
	if err := reloadCatalog(ctx); err != nil {
		log.Fatalf("load catalog: %v", err)
	}
	if cfg.AdminConfigBaseURL != "" && cfg.AdminConfigReloadInterval > 0 {
		go func() {
			ticker := time.NewTicker(cfg.AdminConfigReloadInterval)
			defer ticker.Stop()
			for range ticker.C {
				reloadCtx, reloadCancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := reloadCatalog(reloadCtx); err != nil {
					logger.Error("reload catalog from admin failed", "error", err)
				}
				reloadCancel()
			}
		}()
	}

	server := httpapi.NewServer(cfg, pgStore, manager, reloadCatalog, logger)

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("llm gateway listening", "addr", cfg.ListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen and serve: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown: %v", err)
	}
}
