package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"llm-gateway/internal/config"
	"llm-gateway/internal/store"
)

func TestApplyCatalog(t *testing.T) {
	t.Setenv("TEST_OPENAI_KEY", "sk-test")
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.json")
	err := os.WriteFile(path, []byte(`{
  "keys": [{"name":"openai-main","value_env":"TEST_OPENAI_KEY"}],
  "providers": [
    {"name":"default-mock","type":"mock","enabled":true,"is_default":false},
    {"name":"openai-default","type":"openai","base_url":"https://api.openai.com/v1","api_key_from":"openai-main","enabled":true,"is_default":true}
  ],
  "models": [
    {"name":"mock-chat","provider":"default-mock","upstream_model":"mock-chat","owned_by":"mock","enabled":true},
    {"name":"gpt-4o-mini","provider":"openai-default","upstream_model":"gpt-4o-mini","owned_by":"openai","enabled":true}
  ]
}`), 0o644)
	if err != nil {
		t.Fatalf("write catalog: %v", err)
	}

	st := store.NewMemoryStore()
	models, err := config.ApplyCatalog(context.Background(), st, path)
	if err != nil {
		t.Fatalf("ApplyCatalog error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	providers, err := st.ListProviders(context.Background())
	if err != nil {
		t.Fatalf("ListProviders error: %v", err)
	}
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
	if providers[0].Name != "openai-default" && providers[1].Name != "openai-default" {
		t.Fatalf("openai provider not loaded: %+v", providers)
	}
}
