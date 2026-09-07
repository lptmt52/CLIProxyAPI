package cliproxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestProbeOpenAICompatibilityProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != healthProbePath {
			t.Errorf("path = %q, want %q", r.URL.Path, healthProbePath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer provider-key" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-Provider-Header"); got != "provider-value" {
			t.Errorf("X-Provider-Header = %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := probeOpenAICompatibilityProvider(context.Background(), config.OpenAICompatibility{
		BaseURL:       server.URL,
		Headers:       map[string]string{"X-Provider-Header": "provider-value"},
		APIKeyEntries: []config.OpenAICompatibilityAPIKey{{APIKey: "provider-key"}},
	}, &config.Config{})
	if err != nil {
		t.Fatalf("probeOpenAICompatibilityProvider() error = %v", err)
	}
}

func TestHealthProbeInterval(t *testing.T) {
	if got := healthProbeInterval(config.OpenAICompatibility{}); got != defaultHealthProbeInterval {
		t.Fatalf("default interval = %s, want %s", got, defaultHealthProbeInterval)
	}
	provider := config.OpenAICompatibility{HealthProbeIntervalSeconds: 17}
	if got := healthProbeInterval(provider); got != 17*time.Second {
		t.Fatalf("configured interval = %s, want 17s", got)
	}
}

func TestProviderHealthProbeSignatureChangesForProbeInputs(t *testing.T) {
	provider := config.OpenAICompatibility{
		Name:    "provider",
		BaseURL: "https://example.com/v1",
		Headers: map[string]string{"X-Test": "one"},
		APIKeyEntries: []config.OpenAICompatibilityAPIKey{{
			APIKey:   "key",
			ProxyURL: "direct",
		}},
		HealthProbeIntervalSeconds: 10,
	}
	signature := providerHealthProbeSignature(provider)
	provider.Headers["X-Test"] = "two"
	if signature == providerHealthProbeSignature(provider) {
		t.Fatal("signature did not change after header update")
	}
	provider.Headers["X-Test"] = "one"
	provider.HealthProbeIntervalSeconds = 11
	if signature == providerHealthProbeSignature(provider) {
		t.Fatal("signature did not change after interval update")
	}
}

func TestProviderHealthProbeDisablesAfterRecovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configYAML := "openai-compatibility:\n" +
		"  - name: provider-a\n" +
		"    base-url: " + server.URL + "\n" +
		"    health-probe-enabled: true\n" +
		"    health-probe-interval-seconds: 1\n"
	if errWrite := os.WriteFile(configPath, []byte(configYAML), 0o600); errWrite != nil {
		t.Fatalf("write config: %v", errWrite)
	}
	cfg := &config.Config{OpenAICompatibility: []config.OpenAICompatibility{{
		Name:                       "provider-a",
		BaseURL:                    server.URL,
		HealthProbeEnabled:         true,
		HealthProbeIntervalSeconds: 1,
	}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &Service{cfg: cfg, configPath: configPath}
	s.startProviderHealthProbes(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !cfg.OpenAICompatibility[0].HealthProbeEnabled {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if cfg.OpenAICompatibility[0].HealthProbeEnabled {
		t.Fatal("health probe remained enabled after successful recovery")
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.providerHealthMu.Lock()
		probeCount := len(s.providerHealthProbes)
		s.providerHealthMu.Unlock()
		if probeCount == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	s.providerHealthMu.Lock()
	probeCount := len(s.providerHealthProbes)
	s.providerHealthMu.Unlock()
	if probeCount != 0 {
		t.Fatalf("provider health probes = %d, want 0", probeCount)
	}

	saved, errLoad := config.LoadConfig(configPath)
	if errLoad != nil {
		t.Fatalf("load saved config: %v", errLoad)
	}
	if len(saved.OpenAICompatibility) != 1 || saved.OpenAICompatibility[0].HealthProbeEnabled {
		t.Fatalf("saved health probe = %#v, want one disabled provider", saved.OpenAICompatibility)
	}
	s.stopProviderHealthProbes()
}
