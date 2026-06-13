package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"llm-gateway/internal/gateway"
	"llm-gateway/internal/store"
)

type Catalog struct {
	Keys      []CatalogKey      `json:"keys"`
	Providers []CatalogProvider `json:"providers"`
	Models    []CatalogModel    `json:"models"`
}

type CatalogKey struct {
	Name     string `json:"name"`
	Value    string `json:"value,omitempty"`
	ValueEnv string `json:"value_env,omitempty"`
}

type CatalogProvider struct {
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	BaseURL       string            `json:"base_url,omitempty"`
	APIKey        string            `json:"api_key,omitempty"`
	APIKeyFrom    string            `json:"api_key_from,omitempty"`
	ModelPrefixes []string          `json:"model_prefixes,omitempty"`
	Enabled       bool              `json:"enabled"`
	IsDefault     bool              `json:"is_default"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type CatalogModel struct {
	Name                      string  `json:"name"`
	Provider                  string  `json:"provider"`
	UpstreamModel             string  `json:"upstream_model,omitempty"`
	OwnedBy                   string  `json:"owned_by,omitempty"`
	PromptCostPer1KTokens     float64 `json:"prompt_cost_per_1k_tokens,omitempty"`
	CompletionCostPer1KTokens float64 `json:"completion_cost_per_1k_tokens,omitempty"`
	Enabled                   bool    `json:"enabled"`
}

func LoadCatalog(path string) (Catalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, err
	}
	var c Catalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return Catalog{}, fmt.Errorf("parse catalog: %w", err)
	}
	return c, nil
}

func LoadCatalogFromURL(ctx context.Context, url string) (Catalog, error) {
	if strings.TrimSpace(url) == "" {
		return Catalog{}, fmt.Errorf("catalog url is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Catalog{}, fmt.Errorf("build catalog request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Catalog{}, fmt.Errorf("request catalog: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return Catalog{}, fmt.Errorf("request catalog failed: status=%d", resp.StatusCode)
	}
	var c Catalog
	if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
		return Catalog{}, fmt.Errorf("decode catalog: %w", err)
	}
	return c, nil
}

func ApplyCatalog(ctx context.Context, st store.Store, path string) ([]gateway.ModelRoute, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	catalog, err := LoadCatalog(path)
	if err != nil {
		return nil, err
	}
	return ApplyCatalogData(ctx, st, catalog)
}

func ApplyCatalogFromURL(ctx context.Context, st store.Store, url string) ([]gateway.ModelRoute, error) {
	catalog, err := LoadCatalogFromURL(ctx, url)
	if err != nil {
		return nil, err
	}
	return ApplyCatalogData(ctx, st, catalog)
}

func ApplyCatalogData(ctx context.Context, st store.Store, catalog Catalog) ([]gateway.ModelRoute, error) {
	keys := map[string]string{}
	for _, item := range catalog.Keys {
		if strings.TrimSpace(item.Name) == "" {
			return nil, fmt.Errorf("catalog key name is required")
		}
		value := strings.TrimSpace(item.Value)
		if value == "" && strings.TrimSpace(item.ValueEnv) != "" {
			value = strings.TrimSpace(os.Getenv(strings.TrimSpace(item.ValueEnv)))
		}
		if value == "" {
			return nil, fmt.Errorf("catalog key %q has empty value", item.Name)
		}
		keys[item.Name] = value
	}

	providers := make([]gateway.ProviderConfig, 0, len(catalog.Providers))
	for _, item := range catalog.Providers {
		if strings.TrimSpace(item.Name) == "" {
			return nil, fmt.Errorf("catalog provider name is required")
		}
		apiKey := strings.TrimSpace(item.APIKey)
		if apiKey == "" && strings.TrimSpace(item.APIKeyFrom) != "" {
			resolved, ok := keys[item.APIKeyFrom]
			if !ok {
				return nil, fmt.Errorf("catalog provider %q references unknown key %q", item.Name, item.APIKeyFrom)
			}
			apiKey = resolved
		}
		providers = append(providers, gateway.ProviderConfig{
			Name:          item.Name,
			Type:          strings.ToLower(strings.TrimSpace(item.Type)),
			BaseURL:       strings.TrimSpace(item.BaseURL),
			APIKey:        apiKey,
			ModelPrefixes: sanitizePrefixes(item.ModelPrefixes),
			Enabled:       item.Enabled,
			IsDefault:     item.IsDefault,
			Metadata:      item.Metadata,
		})
	}

	existing, err := st.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	expected := map[string]gateway.ProviderConfig{}
	for _, provider := range providers {
		expected[provider.Name] = provider
		if err := st.UpsertProvider(ctx, provider); err != nil {
			return nil, err
		}
	}
	for _, current := range existing {
		if _, ok := expected[current.Name]; !ok {
			if err := st.DeleteProvider(ctx, current.Name); err != nil {
				return nil, err
			}
		}
	}

	models := make([]gateway.ModelRoute, 0, len(catalog.Models))
	for _, item := range catalog.Models {
		if strings.TrimSpace(item.Name) == "" || strings.TrimSpace(item.Provider) == "" {
			return nil, fmt.Errorf("catalog model name/provider is required")
		}
		upstreamModel := strings.TrimSpace(item.UpstreamModel)
		if upstreamModel == "" {
			upstreamModel = item.Name
		}
		ownedBy := strings.TrimSpace(item.OwnedBy)
		if ownedBy == "" {
			ownedBy = item.Provider
		}
		models = append(models, gateway.ModelRoute{
			Name:                      item.Name,
			Provider:                  item.Provider,
			UpstreamModel:             upstreamModel,
			OwnedBy:                   ownedBy,
			PromptCostPer1KTokens:     item.PromptCostPer1KTokens,
			CompletionCostPer1KTokens: item.CompletionCostPer1KTokens,
			Enabled:                   item.Enabled,
		})
	}
	return models, nil
}

func sanitizePrefixes(prefixes []string) []string {
	out := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		p := strings.TrimSpace(prefix)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	if out == nil {
		return []string{}
	}
	return out
}
