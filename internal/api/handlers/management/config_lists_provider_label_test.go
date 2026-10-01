package management

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
)

func TestPatchGeminiKeyUpdatesProviderLabel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg: &config.Config{
			GeminiKey: []config.GeminiKey{{
				APIKey:  "gemini-key",
				BaseURL: "https://gemini.example.com/v1",
			}},
		},
		configFilePath: writeTestConfigFile(t),
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/gemini-api-key", bytes.NewBufferString(`{"index":0,"value":{"label":"  生产可用  "}}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PatchGeminiKey(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := h.cfg.GeminiKey[0].Label; got != "生产可用" {
		t.Fatalf("label = %q, want %q", got, "生产可用")
	}

	saved, errLoad := config.LoadConfig(h.configFilePath)
	if errLoad != nil {
		t.Fatalf("load saved config: %v", errLoad)
	}
	if len(saved.GeminiKey) != 1 || saved.GeminiKey[0].Label != h.cfg.GeminiKey[0].Label {
		t.Fatalf("saved gemini labels = %#v, want %q", saved.GeminiKey, h.cfg.GeminiKey[0].Label)
	}

	clearRec := httptest.NewRecorder()
	clearContext, _ := gin.CreateTestContext(clearRec)
	clearContext.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/gemini-api-key", bytes.NewBufferString(`{"index":0,"value":{"label":""}}`))
	clearContext.Request.Header.Set("Content-Type", "application/json")
	h.PatchGeminiKey(clearContext)

	if clearRec.Code != http.StatusOK {
		t.Fatalf("clear status = %d, want %d; body=%s", clearRec.Code, http.StatusOK, clearRec.Body.String())
	}
	saved, errLoad = config.LoadConfig(h.configFilePath)
	if errLoad != nil {
		t.Fatalf("reload cleared config: %v", errLoad)
	}
	if len(saved.GeminiKey) != 1 || saved.GeminiKey[0].Label != "" {
		t.Fatalf("cleared gemini labels = %#v, want empty label", saved.GeminiKey)
	}
}

func TestPatchOpenAICompatUpdatesHealthProbeFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg: &config.Config{OpenAICompatibility: []config.OpenAICompatibility{{
			Name:    "provider-a",
			BaseURL: "https://example.com/v1",
		}}},
		configFilePath: writeTestConfigFile(t),
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/openai-compatibility", bytes.NewBufferString(`{"index":0,"value":{"health-probe-enabled":true,"health-probe-interval-seconds":17}}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PatchOpenAICompat(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	entry := h.cfg.OpenAICompatibility[0]
	if !entry.HealthProbeEnabled || entry.HealthProbeIntervalSeconds != 17 {
		t.Fatalf("updated entry = %#v", entry)
	}
	saved, errLoad := config.LoadConfig(h.configFilePath)
	if errLoad != nil {
		t.Fatalf("load saved config: %v", errLoad)
	}
	if len(saved.OpenAICompatibility) != 1 || !saved.OpenAICompatibility[0].HealthProbeEnabled || saved.OpenAICompatibility[0].HealthProbeIntervalSeconds != 17 {
		t.Fatalf("saved entry = %#v", saved.OpenAICompatibility)
	}
}

func TestPatchGeminiKeyUpdatesPriority(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg: &config.Config{
			GeminiKey: []config.GeminiKey{{
				APIKey:  "gemini-key",
				BaseURL: "https://gemini.example.com/v1",
			}},
		},
		configFilePath: writeTestConfigFile(t),
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPatch, "/v0/management/gemini-api-key", bytes.NewBufferString(`{"index":0,"value":{"priority":7}}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PatchGeminiKey(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := h.cfg.GeminiKey[0].Priority; got != 7 {
		t.Fatalf("priority = %d, want 7", got)
	}

	saved, errLoad := config.LoadConfig(h.configFilePath)
	if errLoad != nil {
		t.Fatalf("load saved config: %v", errLoad)
	}
	if len(saved.GeminiKey) != 1 || saved.GeminiKey[0].Priority != 7 {
		t.Fatalf("saved gemini priority = %#v, want 7", saved.GeminiKey)
	}
}
func TestPutGeminiKeysPreservesExistingProviderLabel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &Handler{
		cfg: &config.Config{
			GeminiKey: []config.GeminiKey{{
				APIKey:  "gemini-key",
				BaseURL: "https://gemini.example.com/v1",
				Label:   "专线",
			}},
		},
		configFilePath: writeTestConfigFile(t),
	}

	body, errMarshal := json.Marshal([]config.GeminiKey{{
		APIKey:  "gemini-key",
		BaseURL: "https://gemini.example.com/v1",
		Prefix:  "team",
	}})
	if errMarshal != nil {
		t.Fatalf("marshal request: %v", errMarshal)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/v0/management/gemini-api-key", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.PutGeminiKeys(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := h.cfg.GeminiKey[0].Label; got != "专线" {
		t.Fatalf("label = %q, want %q", got, "专线")
	}
	if got := h.cfg.GeminiKey[0].Prefix; got != "team" {
		t.Fatalf("prefix = %q, want %q", got, "team")
	}
}

func TestProviderLabelPreservationSupportsAllProviderTypes(t *testing.T) {
	claude := []config.ClaudeKey{{APIKey: "claude", BaseURL: "https://claude.example.com/v1", Label: "Claude 标签"}}
	claudeNext := []config.ClaudeKey{{APIKey: "claude", BaseURL: "https://claude.example.com/v1"}}
	preserveClaudeKeyLabels(claude, claudeNext)
	if got := claudeNext[0].Label; got != claude[0].Label {
		t.Fatalf("claude label = %q, want %q", got, claude[0].Label)
	}

	codex := []config.CodexKey{{APIKey: "codex", BaseURL: "https://codex.example.com/v1", Label: "Codex 标签"}}
	codexNext := []config.CodexKey{{APIKey: "codex", BaseURL: "https://codex.example.com/v1"}}
	preserveCodexKeyLabels(codex, codexNext)
	if got := codexNext[0].Label; got != codex[0].Label {
		t.Fatalf("codex label = %q, want %q", got, codex[0].Label)
	}

	vertex := []config.VertexCompatKey{{APIKey: "vertex", BaseURL: "https://vertex.example.com/v1", Label: "Vertex 标签"}}
	vertexNext := []config.VertexCompatKey{{APIKey: "vertex", BaseURL: "https://vertex.example.com/v1"}}
	preserveVertexCompatKeyLabels(vertex, vertexNext)
	if got := vertexNext[0].Label; got != vertex[0].Label {
		t.Fatalf("vertex label = %q, want %q", got, vertex[0].Label)
	}

	openAI := []config.OpenAICompatibility{{Name: "provider", BaseURL: "https://openai.example.com/v1", Label: "OpenAI 标签"}}
	openAINext := []config.OpenAICompatibility{{Name: "provider", BaseURL: "https://openai.example.com/v1"}}
	preserveOpenAICompatibilityLabels(openAI, openAINext)
	if got := openAINext[0].Label; got != openAI[0].Label {
		t.Fatalf("openai label = %q, want %q", got, openAI[0].Label)
	}
}
