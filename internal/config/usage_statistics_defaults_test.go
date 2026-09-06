package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseConfigBytesUsageStatisticsDefaults(t *testing.T) {
	cfgDefault, errDefault := ParseConfigBytes([]byte("debug: false\n"))
	if errDefault != nil {
		t.Fatalf("ParseConfigBytes() default error = %v", errDefault)
	}
	if !cfgDefault.UsageStatisticsEnabled {
		t.Fatal("UsageStatisticsEnabled = false, want true when omitted")
	}

	cfgDisabled, errDisabled := ParseConfigBytes([]byte("usage-statistics-enabled: false\n"))
	if errDisabled != nil {
		t.Fatalf("ParseConfigBytes() disabled error = %v", errDisabled)
	}
	if cfgDisabled.UsageStatisticsEnabled {
		t.Fatal("UsageStatisticsEnabled = true, want false when explicitly disabled")
	}
}

func TestLoadConfigOptionalUsageStatisticsDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if errWrite := os.WriteFile(configPath, []byte("debug: false\n"), 0o600); errWrite != nil {
		t.Fatalf("write default config: %v", errWrite)
	}

	cfgDefault, errDefault := LoadConfigOptional(configPath, false)
	if errDefault != nil {
		t.Fatalf("LoadConfigOptional() default error = %v", errDefault)
	}
	if !cfgDefault.UsageStatisticsEnabled {
		t.Fatal("UsageStatisticsEnabled = false, want true when omitted")
	}

	if errWrite := os.WriteFile(configPath, []byte("usage-statistics-enabled: false\n"), 0o600); errWrite != nil {
		t.Fatalf("write disabled config: %v", errWrite)
	}
	cfgDisabled, errDisabled := LoadConfigOptional(configPath, false)
	if errDisabled != nil {
		t.Fatalf("LoadConfigOptional() disabled error = %v", errDisabled)
	}
	if cfgDisabled.UsageStatisticsEnabled {
		t.Fatal("UsageStatisticsEnabled = true, want false when explicitly disabled")
	}
}
