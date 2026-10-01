package management

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
)

func TestConfigV8PreservesLocalExtensionsAcrossMigrationAndWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	path := filepath.Join(t.TempDir(), "config.yaml")
	legacy := `# Local extensions must survive migration.
log-level: warn
gemini-api-key:
  - api-key: fixture-gemini
    label: local-gemini
    disabled: true
    base-url: https://gemini.example.test
codex-api-key:
  - api-key: fixture-codex
    label: local-codex
    base-url: https://codex.example.test
openai-compatibility:
  - name: local-provider
    label: local-label
    base-url: https://provider.example.test/v1
    health-probe-enabled: true
    health-probe-interval-seconds: 17
    api-key-entries:
      - api-key: fixture-key
    models: [{name: model-a, alias: alias-a}]
`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{cfg: cfg, configFilePath: path}
	router := gin.New()
	router.GET("/v8/management/config", h.ConfigV8)
	router.PATCH("/v8/management/config", h.ConfigV8)
	request := func(method, body string) *httptest.ResponseRecorder {
		t.Helper()
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(method, "/v8/management/config", strings.NewReader(body)))
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", method, rr.Code, rr.Body.String())
		}
		return rr
	}
	read := request(http.MethodGet, "")
	for _, marker := range []string{`"log-level":"warn"`, `"local-label"`, `"health-probe-enabled":true`, `"local-gemini"`} {
		if !strings.Contains(read.Body.String(), marker) {
			t.Fatalf("GET missing %s", marker)
		}
	}
	request(http.MethodPatch, `{"observability":{"logs":{"log-level":"info"}},"routing":{"retry":{"request-retry":2}}}`)
	loaded, err := config.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LogLevel != "info" || loaded.RequestRetry != 2 {
		t.Fatal("v8 fields were not applied")
	}
	if len(loaded.GeminiKey) != 1 || !loaded.GeminiKey[0].Disabled || loaded.GeminiKey[0].Label != "local-gemini" {
		t.Fatal("Gemini customization lost")
	}
	if len(loaded.CodexKey) != 1 || loaded.CodexKey[0].Label != "local-codex" {
		t.Fatal("Codex label lost")
	}
	if len(loaded.OpenAICompatibility) != 1 {
		t.Fatal("provider lost")
	}
	p := loaded.OpenAICompatibility[0]
	if p.Label != "local-label" || !p.HealthProbeEnabled || p.HealthProbeIntervalSeconds != 17 || len(p.APIKeyEntries) != 1 || len(p.Models) != 1 {
		t.Fatal("provider customization lost")
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "config-version: 8") {
		t.Fatal("write did not migrate to v8")
	}
}
