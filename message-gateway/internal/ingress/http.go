package ingress

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"message-gateway/internal/metrics"
	"message-gateway/internal/model"
	"message-gateway/internal/store"
)

type HTTPHandler struct {
	store    *store.PostgresStore
	metrics  *metrics.Registry
	callback http.HandlerFunc
	logger   *slog.Logger
}

func NewHTTPHandler(
	store *store.PostgresStore,
	metrics *metrics.Registry,
	callback http.HandlerFunc,
	logger *slog.Logger,
) *HTTPHandler {
	return &HTTPHandler{
		store:    store,
		metrics:  metrics,
		callback: callback,
		logger:   logger,
	}
}

func (h *HTTPHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /callbacks/feishu", h.handleLarkCallback)
	mux.HandleFunc("GET /admin/healthz", h.handleHealthz)
	mux.HandleFunc("GET /admin/metrics", h.handleMetrics)
	mux.HandleFunc("GET /admin/jobs/dead", h.handleDeadJobs)
	mux.HandleFunc("POST /admin/jobs/{job_id}/replay", h.handleReplayJob)
}

func (h *HTTPHandler) handleLarkCallback(w http.ResponseWriter, r *http.Request) {
	h.metrics.IncInboundRequests()
	h.callback(w, r)
}

func (h *HTTPHandler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.store.Ping(ctx); err != nil {
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *HTTPHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(h.metrics.RenderPrometheus()))
}

func (h *HTTPHandler) handleDeadJobs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 1000 {
			limit = v
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	jobs, err := h.store.ListDeadJobs(ctx, limit)
	if err != nil {
		h.logger.Error("list dead jobs failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"items": jobs})
}

func (h *HTTPHandler) handleReplayJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("job_id")
	if jobID == "" {
		http.Error(w, "missing job_id", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	ok, err := h.store.ReplayDeadJob(ctx, jobID)
	if err != nil {
		h.logger.Error("replay dead job failed", "job_id", jobID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "job not found or not dead", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"job_id": jobID, "status": model.JobStatusPending})
}
