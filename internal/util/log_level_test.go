package util

import (
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/internal/config"
	log "github.com/sirupsen/logrus"
)

func TestConfiguredLogLevel(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want log.Level
	}{
		{name: "legacy info", cfg: &config.Config{}, want: log.InfoLevel},
		{name: "legacy debug", cfg: &config.Config{Debug: true}, want: log.DebugLevel},
		{name: "explicit trace overrides debug", cfg: &config.Config{Debug: false, LogLevel: "trace"}, want: log.TraceLevel},
		{name: "explicit warn overrides debug", cfg: &config.Config{Debug: true, LogLevel: " warn "}, want: log.WarnLevel},
		{name: "invalid falls back to debug", cfg: &config.Config{Debug: true, LogLevel: "invalid"}, want: log.DebugLevel},
		{name: "nil", cfg: nil, want: log.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConfiguredLogLevel(tt.cfg); got != tt.want {
				t.Fatalf("ConfiguredLogLevel() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestMaskSensitiveHeaderValueCookie(t *testing.T) {
	if got := MaskSensitiveHeaderValue("Cookie", "session=secret-value"); got == "session=secret-value" {
		t.Fatal("Cookie header should be masked")
	}
}
