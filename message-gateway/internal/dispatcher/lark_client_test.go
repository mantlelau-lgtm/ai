package dispatcher

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"message-gateway/internal/config"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

func TestNewLarkClientSupportsAppIDAndBotIDLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"bots": [
				{
					"bot_id":"FamilyHealth",
					"app_id":"cli_a941d0ed9bb95bb3",
					"app_secret":"secret-1",
					"open_base_url":"https://open.feishu.cn/"
				}
			]
		}`))
	}))
	defer srv.Close()

	client := NewLarkClient(config.Config{
		AdminConfigBaseURL:   srv.URL,
		AdminMessageBotsPath: "",
		LarkOpenBaseURL:      "https://open.feishu.cn/",
	})

	if _, err := client.clientFor("FamilyHealth"); err != nil {
		t.Fatalf("expected bot_id lookup to succeed, got err=%v", err)
	}
	if _, err := client.clientFor("cli_a941d0ed9bb95bb3"); err != nil {
		t.Fatalf("expected app_id lookup to succeed, got err=%v", err)
	}
}

func TestLarkClientReloadsCredentialsFromAdminRuntime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"bots": [
				{
					"bot_id":"FamilyHealth",
					"app_id":"cli_a941d0ed9bb95bb3",
					"app_secret":"secret-1",
					"open_base_url":"https://open.feishu.cn/"
				}
			]
		}`))
	}))
	defer srv.Close()

	client := &LarkClient{
		cfg: config.Config{
			AdminConfigBaseURL:   srv.URL,
			AdminMessageBotsPath: "",
			LarkOpenBaseURL:      "https://open.feishu.cn/",
		},
		clients: map[string]*lark.Client{},
	}

	if _, err := client.clientFor("cli_a941d0ed9bb95bb3"); err != nil {
		t.Fatalf("expected lazy reload lookup to succeed, got err=%v", err)
	}
}
