package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr                  string
	DatabaseURL               string
	AdminConfigBaseURL        string
	AdminMessageBotsPath      string
	AdminMessageRoutesPath    string
	AgentCenterBaseURL        string
	AgentCenterAgentsPath     string
	LarkAppID                 string
	LarkAppSecret             string
	LarkVerificationToken     string
	LarkEncryptKey            string
	LarkOpenBaseURL           string
	LarkBotsPath              string
	LLMGatewayBaseURL         string
	LLMGatewayChatPath        string
	AgentSelectorModel        string
	AgentSelectorKeyName      string
	AgentSelectorTimeout      time.Duration
	AgentUnavailableReplyText string
	LarkWSEnabled             bool
	LarkStreamingCardEnabled  bool
	LarkStreamingCardUpdate   time.Duration
	LarkStreamingCardMaxBytes int
	CoreBaseURL               string
	CoreStreamPath            string
	CoreTimeout               time.Duration
	RouteRulesPath            string
	RouteRulesReloadInterval  time.Duration
	WorkerPollInterval        time.Duration
	WorkerBatchSize           int
	WorkerMaxAttempts         int
	WorkerRetryBaseInterval   time.Duration
	WorkerJobTimeout          time.Duration
}

func Load() Config {
	return Config{
		HTTPAddr:                  getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:               getenv("DATABASE_URL", "postgres://postgres:postgres123@127.0.0.1:5432/message_gateway?sslmode=disable"),
		AdminConfigBaseURL:        strings.TrimRight(os.Getenv("ADMIN_CONFIG_BASE_URL"), "/"),
		AdminMessageBotsPath:      getenv("ADMIN_MESSAGE_BOTS_PATH", "/api/runtime/message-gateway/bots"),
		AdminMessageRoutesPath:    getenv("ADMIN_MESSAGE_ROUTES_PATH", "/api/runtime/message-gateway/routes"),
		AgentCenterBaseURL:        strings.TrimRight(os.Getenv("AGENT_CENTER_BASE_URL"), "/"),
		AgentCenterAgentsPath:     getenv("AGENT_CENTER_AGENTS_PATH", "/api/runtime/agents"),
		LarkAppID:                 os.Getenv("LARK_APP_ID"),
		LarkAppSecret:             os.Getenv("LARK_APP_SECRET"),
		LarkVerificationToken:     os.Getenv("LARK_VERIFICATION_TOKEN"),
		LarkEncryptKey:            os.Getenv("LARK_ENCRYPT_KEY"),
		LarkOpenBaseURL:           getenv("LARK_OPEN_BASE_URL", "https://open.larksuite.com"),
		LarkBotsPath:              os.Getenv("LARK_BOTS_PATH"),
		LLMGatewayBaseURL:         strings.TrimRight(os.Getenv("LLM_GATEWAY_BASE_URL"), "/"),
		LLMGatewayChatPath:        getenv("LLM_GATEWAY_CHAT_PATH", "/v1/chat/completions"),
		AgentSelectorModel:        os.Getenv("AGENT_SELECTOR_MODEL"),
		AgentSelectorKeyName:      os.Getenv("AGENT_SELECTOR_KEY_NAME"),
		AgentSelectorTimeout:      getDuration("AGENT_SELECTOR_TIMEOUT", 20*time.Second),
		AgentUnavailableReplyText: getenv("AGENT_UNAVAILABLE_REPLY_TEXT", "当前没有可用的 agent，请稍后再试。"),
		LarkWSEnabled:             getBool("LARK_WS_ENABLED", false),
		LarkStreamingCardEnabled:  getBool("LARK_STREAMING_CARD_ENABLED", true),
		LarkStreamingCardUpdate:   getDuration("LARK_STREAMING_CARD_UPDATE_INTERVAL", 400*time.Millisecond),
		LarkStreamingCardMaxBytes: getInt("LARK_STREAMING_CARD_MAX_BYTES", 20*1024),
		CoreBaseURL:               os.Getenv("CORE_BASE_URL"),
		CoreStreamPath:            getenv("CORE_STREAM_PATH", "/v1/messages:stream"),
		CoreTimeout:               getDuration("CORE_TIMEOUT", 60*time.Second),
		RouteRulesPath:            os.Getenv("ROUTE_RULES_PATH"),
		RouteRulesReloadInterval:  getDuration("ROUTE_RULES_RELOAD_INTERVAL", 2*time.Second),
		WorkerPollInterval:        getDuration("WORKER_POLL_INTERVAL", 2*time.Second),
		WorkerBatchSize:           getInt("WORKER_BATCH_SIZE", 10),
		WorkerMaxAttempts:         getInt("WORKER_MAX_ATTEMPTS", 8),
		WorkerRetryBaseInterval:   getDuration("WORKER_RETRY_BASE_INTERVAL", 5*time.Second),
		WorkerJobTimeout:          getDuration("WORKER_JOB_TIMEOUT", 600*time.Second),
	}
}

func getBool(key string, defaultVal bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultVal
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return defaultVal
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}
