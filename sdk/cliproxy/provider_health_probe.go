package cliproxy

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/proxyutil"
	log "github.com/sirupsen/logrus"
)

const (
	defaultHealthProbeInterval = 30 * time.Second
	healthProbePath            = "/models"
)

type providerHealthProbe struct {
	cancel     context.CancelFunc
	signature  string
	generation uint64
}

func (s *Service) startProviderHealthProbes(parent context.Context) {
	if s == nil {
		return
	}
	if parent == nil {
		parent = context.Background()
	}

	s.providerHealthMu.Lock()
	if s.providerHealthCancel != nil {
		s.providerHealthCancel()
	}
	for key, probe := range s.providerHealthProbes {
		probe.cancel()
		delete(s.providerHealthProbes, key)
	}
	ctx, cancel := context.WithCancel(parent)
	s.providerHealthCancel = cancel
	s.providerHealthCtx = ctx
	if s.providerHealthProbes == nil {
		s.providerHealthProbes = make(map[string]providerHealthProbe)
	}
	s.providerHealthMu.Unlock()

	s.reconcileProviderHealthProbes()
}

func (s *Service) stopProviderHealthProbes() {
	if s == nil {
		return
	}
	s.providerHealthMu.Lock()
	if s.providerHealthCancel != nil {
		s.providerHealthCancel()
	}
	s.providerHealthCancel = nil
	s.providerHealthCtx = nil
	for key, probe := range s.providerHealthProbes {
		probe.cancel()
		delete(s.providerHealthProbes, key)
	}
	s.providerHealthMu.Unlock()
}

func (s *Service) reconcileProviderHealthProbes() {
	if s == nil {
		return
	}
	s.cfgMu.RLock()
	cfg := s.cfg
	s.cfgMu.RUnlock()

	desired := make(map[string]config.OpenAICompatibility)
	if cfg != nil {
		for _, provider := range cfg.OpenAICompatibility {
			name := strings.TrimSpace(provider.Name)
			if name == "" || provider.Disabled || !provider.HealthProbeEnabled || strings.TrimSpace(provider.BaseURL) == "" {
				continue
			}
			desired[strings.ToLower(name)] = provider
		}
	}

	s.providerHealthMu.Lock()
	defer s.providerHealthMu.Unlock()
	if s.providerHealthCtx == nil {
		return
	}
	if s.providerHealthProbes == nil {
		s.providerHealthProbes = make(map[string]providerHealthProbe)
	}
	for key, probe := range s.providerHealthProbes {
		provider, ok := desired[key]
		if !ok || providerHealthProbeSignature(provider) != probe.signature {
			probe.cancel()
			delete(s.providerHealthProbes, key)
		}
	}
	for key, provider := range desired {
		signature := providerHealthProbeSignature(provider)
		if probe, ok := s.providerHealthProbes[key]; ok && probe.signature == signature {
			continue
		}
		ctx, cancel := context.WithCancel(s.providerHealthCtx)
		s.providerHealthGeneration++
		generation := s.providerHealthGeneration
		s.providerHealthProbes[key] = providerHealthProbe{cancel: cancel, signature: signature, generation: generation}
		go s.runProviderHealthProbe(ctx, key, provider, signature, generation)
		log.Infof("provider health probe started: provider=%s interval=%s", provider.Name, healthProbeInterval(provider))
	}
}

func providerHealthProbeSignature(provider config.OpenAICompatibility) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(provider.Name)),
		strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/"),
		fmt.Sprintf("%t", provider.Disabled),
		fmt.Sprintf("%d", provider.HealthProbeIntervalSeconds),
	}
	for _, entry := range provider.APIKeyEntries {
		parts = append(parts, "key="+strings.TrimSpace(entry.APIKey), "proxy="+strings.TrimSpace(entry.ProxyURL))
	}
	headers := make([]string, 0, len(provider.Headers))
	for key, value := range provider.Headers {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey != "" {
			headers = append(headers, trimmedKey+"\x00"+value)
		}
	}
	sort.Strings(headers)
	for _, header := range headers {
		parts = append(parts, "header="+header)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", sum[:])
}

func healthProbeInterval(provider config.OpenAICompatibility) time.Duration {
	if provider.HealthProbeIntervalSeconds > 0 {
		return time.Duration(provider.HealthProbeIntervalSeconds) * time.Second
	}
	return defaultHealthProbeInterval
}

func (s *Service) runProviderHealthProbe(ctx context.Context, key string, provider config.OpenAICompatibility, signature string, generation uint64) {
	defer s.removeProviderHealthProbe(key, signature, generation)
	interval := healthProbeInterval(provider)
	for {
		if errProbe := probeOpenAICompatibilityProvider(ctx, provider, s.currentConfig()); errProbe == nil {
			if errDisable := s.disableProviderHealthProbe(provider.Name, signature, generation); errDisable != nil {
				log.WithError(errDisable).WithField("provider", provider.Name).Warn("provider recovered but health probe flag could not be cleared")
				if !s.providerHealthProbeStillCurrent(ctx, key, signature, generation) {
					return
				}
			} else {
				log.Infof("provider recovered; health probe stopped: provider=%s", provider.Name)
				return
			}
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (s *Service) providerHealthProbeStillCurrent(ctx context.Context, key, signature string, generation uint64) bool {
	if ctx.Err() != nil {
		return false
	}
	s.providerHealthMu.Lock()
	defer s.providerHealthMu.Unlock()
	probe, ok := s.providerHealthProbes[key]
	return ok && probe.signature == signature && probe.generation == generation
}

func (s *Service) removeProviderHealthProbe(key, signature string, generation uint64) {
	if s == nil {
		return
	}
	s.providerHealthMu.Lock()
	defer s.providerHealthMu.Unlock()
	probe, ok := s.providerHealthProbes[key]
	if ok && probe.signature == signature && probe.generation == generation {
		delete(s.providerHealthProbes, key)
	}
}

func (s *Service) currentConfig() *config.Config {
	if s == nil {
		return nil
	}
	s.cfgMu.RLock()
	defer s.cfgMu.RUnlock()
	return s.cfg
}

func probeOpenAICompatibilityProvider(ctx context.Context, provider config.OpenAICompatibility, cfg *config.Config) error {
	baseURL := strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
	if baseURL == "" {
		return fmt.Errorf("provider base-url is empty")
	}
	probeURL := baseURL + healthProbePath
	entries := provider.APIKeyEntries
	if len(entries) == 0 {
		entries = []config.OpenAICompatibilityAPIKey{{}}
	}
	var lastErr error
	for _, entry := range entries {
		if errProbe := probeOpenAICompatibilityCredential(ctx, cfg, probeURL, provider.Headers, entry); errProbe == nil {
			return nil
		} else {
			lastErr = errProbe
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("provider did not respond successfully")
	}
	return lastErr
}

func probeOpenAICompatibilityCredential(ctx context.Context, cfg *config.Config, probeURL string, headers map[string]string, entry config.OpenAICompatibilityAPIKey) error {
	req, errNew := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if errNew != nil {
		return errNew
	}
	req.Header.Set("Accept", "application/json")
	if key := strings.TrimSpace(entry.APIKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	for key, value := range headers {
		if strings.TrimSpace(key) != "" {
			req.Header.Set(key, value)
		}
	}

	client := &http.Client{}
	var transport *http.Transport
	proxyURL := strings.TrimSpace(entry.ProxyURL)
	if proxyURL == "" && cfg != nil {
		proxyURL = strings.TrimSpace(cfg.ProxyURL)
	}
	if proxyURL != "" {
		var errTransport error
		transport, _, errTransport = proxyutil.BuildHTTPTransport(proxyURL)
		if errTransport != nil {
			return fmt.Errorf("build proxy transport: %w", errTransport)
		}
		if transport != nil {
			client.Transport = transport
		}
	}
	if transport != nil {
		defer transport.CloseIdleConnections()
	}

	resp, errDo := client.Do(req)
	if errDo != nil {
		return errDo
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("provider probe returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (s *Service) disableProviderHealthProbe(providerName, expectedSignature string, expectedGeneration uint64) error {
	if s == nil {
		return nil
	}
	name := strings.TrimSpace(providerName)
	s.configUpdateMu.Lock()
	defer s.configUpdateMu.Unlock()
	if !s.providerHealthProbeStillCurrent(context.Background(), strings.ToLower(name), expectedSignature, expectedGeneration) {
		return nil
	}

	s.cfgMu.Lock()
	if s.cfg == nil {
		s.cfgMu.Unlock()
		return nil
	}
	cfg := s.cfg
	changed := false
	updatedCfg := *cfg
	updatedCfg.OpenAICompatibility = append([]config.OpenAICompatibility(nil), cfg.OpenAICompatibility...)
	for i := range updatedCfg.OpenAICompatibility {
		entry := updatedCfg.OpenAICompatibility[i]
		if strings.EqualFold(strings.TrimSpace(entry.Name), name) &&
			entry.HealthProbeEnabled && providerHealthProbeSignature(entry) == expectedSignature {
			updatedCfg.OpenAICompatibility[i].HealthProbeEnabled = false
			changed = true
			break
		}
	}
	s.cfgMu.Unlock()
	if !changed || strings.TrimSpace(s.configPath) == "" {
		if changed {
			s.cfgMu.Lock()
			if s.cfg == cfg {
				*cfg = updatedCfg
			}
			s.cfgMu.Unlock()
			s.reconcileProviderHealthProbes()
		}
		return nil
	}
	if errSave := config.SaveConfigPreserveComments(s.configPath, &updatedCfg); errSave != nil {
		return fmt.Errorf("save config: %w", errSave)
	}
	s.cfgMu.Lock()
	if s.cfg == cfg {
		*cfg = updatedCfg
	}
	s.cfgMu.Unlock()
	s.reconcileProviderHealthProbes()
	return nil
}
