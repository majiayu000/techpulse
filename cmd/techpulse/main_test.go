package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/majiayu000/techpulse/internal/logger"
	"github.com/majiayu000/techpulse/internal/techpulse"
)

func TestMain(m *testing.M) {
	logger.SetDefault(logger.NewNopLogger())
	log = logger.Default()
	os.Exit(m.Run())
}

// resetGlobals clears the package-level state that parseFlags mutates so
// table-driven cases stay independent of execution order.
func resetGlobals(t *testing.T) {
	t.Helper()
	daemon, quiet, showProgress, showVersion, validateOnly = false, false, false, false, false
	interval = 0
	configPath = ""
	cliOpts = cliFlags{}
}

// writeConfig writes yaml content to a temp file and returns its path.
func writeConfig(t *testing.T, yaml string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "techpulse.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// TestParseFlagsCLIPassedFlagsBeatFileConfig is the regression suite for the
// precedence bug where CLI flag defaults clobbered the config file. Only
// flags explicitly passed on the command line may override file values.
func TestParseFlagsCLIPassedFlagsBeatFileConfig(t *testing.T) {
	tests := []struct {
		name          string
		configYAML    string
		args          []string
		wantLimit     int
		wantTimeout   int
		wantSummary   bool
		wantSources   []string
		wantRetention *int
	}{
		{
			name:        "file enable_summary=true survives default --summary=false",
			configYAML:  "enable_summary: true\n",
			wantLimit:   30,
			wantTimeout: 60,
			wantSummary: true,
		},
		{
			name:        "explicit --summary beats file enable_summary=false",
			configYAML:  "enable_summary: false\n",
			args:        []string{"--summary"},
			wantLimit:   30,
			wantTimeout: 60,
			wantSummary: true,
		},
		{
			name:        "explicit --summary=false beats file enable_summary=true",
			configYAML:  "enable_summary: true\n",
			args:        []string{"--summary=false"},
			wantLimit:   30,
			wantTimeout: 60,
			wantSummary: false,
		},
		{
			name:        "file values apply when no flags passed",
			configYAML:  "limit: 50\ntimeout: 90\nsources:\n  - hackernews_top",
			wantLimit:   50,
			wantTimeout: 90,
			wantSources: []string{"hackernews_top"},
		},
		{
			name:        "explicit --limit beats file limit",
			configYAML:  "limit: 50\n",
			args:        []string{"--limit", "10"},
			wantLimit:   10,
			wantTimeout: 60,
		},
		{
			name:        "explicit --sources beats file sources",
			configYAML:  "sources:\n  - hackernews_top",
			args:        []string{"--sources", "rss, reddit"},
			wantLimit:   30,
			wantTimeout: 60,
			wantSources: []string{"rss", "reddit"},
		},
		{
			name:        "explicit empty --sources clears file sources",
			configYAML:  "sources:\n  - hackernews_top",
			args:        []string{"--sources="},
			wantLimit:   30,
			wantTimeout: 60,
			wantSources: []string{},
		},
		{
			name:          "file retention_days=0 disables cleanup",
			configYAML:    "retention_days: 0\n",
			wantLimit:     30,
			wantTimeout:   60,
			wantRetention: intPtr(0),
		},
		{
			name:          "explicit --retention-days beats file",
			configYAML:    "retention_days: 7\n",
			args:          []string{"--retention-days", "0"},
			wantLimit:     30,
			wantTimeout:   60,
			wantRetention: intPtr(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobals(t)
			args := append([]string{"--config", writeConfig(t, tt.configYAML)}, tt.args...)
			cfg, err := parseFlags(args)
			if err != nil {
				t.Fatalf("parseFlags(%v) returned error: %v", args, err)
			}
			if cfg == nil {
				t.Fatal("parseFlags returned nil config")
			}
			if cfg.Limit != tt.wantLimit {
				t.Errorf("Limit = %d, want %d", cfg.Limit, tt.wantLimit)
			}
			if cfg.Timeout != tt.wantTimeout {
				t.Errorf("Timeout = %d, want %d", cfg.Timeout, tt.wantTimeout)
			}
			if cfg.EnableSummary != tt.wantSummary {
				t.Errorf("EnableSummary = %v, want %v", cfg.EnableSummary, tt.wantSummary)
			}
			if tt.wantSources != nil && !slices.Equal(cfg.Sources, tt.wantSources) {
				t.Errorf("Sources = %v, want %v", cfg.Sources, tt.wantSources)
			}
			if tt.wantRetention != nil && cfg.RetentionDays != *tt.wantRetention {
				t.Errorf("RetentionDays = %d, want %d", cfg.RetentionDays, *tt.wantRetention)
			}
		})
	}
}

func intPtr(v int) *int { return &v }

// TestParseFlagsValidateIsHonest ensures --validate reports failure when the
// config file cannot be loaded/parsed or violates constraints, instead of
// printing success based on silently-fallen-back defaults.
func TestParseFlagsValidateIsHonest(t *testing.T) {
	tests := []struct {
		name            string
		configYAML      string
		skipWrite       bool
		args            []string
		wantErrContains string // empty means expect success: (nil, nil)
	}{
		{
			name:            "missing config file fails instead of validating defaults",
			skipWrite:       true,
			args:            []string{"--validate"},
			wantErrContains: "load config file",
		},
		{
			name:            "malformed YAML fails",
			configYAML:      "limit: [unclosed\n",
			args:            []string{"--validate"},
			wantErrContains: "load config file",
		},
		{
			name:            "out-of-range limit fails",
			configYAML:      "limit: 9999\n",
			args:            []string{"--validate"},
			wantErrContains: "validation failed",
		},
		{
			name:            "unknown source in file fails",
			configYAML:      "sources:\n  - twitter_timeline\n",
			args:            []string{"--validate"},
			wantErrContains: "unknown source",
		},
		{
			name:            "invalid rss feed url fails validate",
			configYAML:      "rss_feeds:\n  - name: X\n    url: notaurl\n",
			args:            []string{"--validate"},
			wantErrContains: "must be a valid HTTP/HTTPS URL",
		},
		{
			name:            "valid config passes",
			configYAML:      "limit: 20\nenable_summary: true\n",
			args:            []string{"--validate"},
			wantErrContains: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobals(t)
			var path string
			if !tt.skipWrite {
				path = writeConfig(t, tt.configYAML)
			} else {
				path = filepath.Join(t.TempDir(), "missing.yaml")
			}
			cfg, err := parseFlags(append([]string{"--config", path}, tt.args...))
			if tt.wantErrContains == "" {
				if err != nil {
					t.Fatalf("expected success, got error: %v", err)
				}
				if cfg != nil {
					t.Errorf("--validate should return nil config, got %+v", cfg)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got success (cfg=%+v)", tt.wantErrContains, cfg)
			}
			if !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrContains)
			}
		})
	}
}

// TestParseFlagsRejections covers inputs that must fail startup with a clear
// error instead of a warning-plus-fallback.
func TestParseFlagsRejections(t *testing.T) {
	tests := []struct {
		name            string
		configYAML      string
		args            []string
		wantErrContains string
	}{
		{
			name:            "invalid interval rejected",
			args:            []string{"--interval", "not-a-duration"},
			wantErrContains: "invalid interval",
		},
		{
			name:            "non-positive interval rejected",
			args:            []string{"--daemon", "--interval", "0s"},
			wantErrContains: "must be positive",
		},
		{
			name:            "negative interval rejected",
			args:            []string{"--interval", "-1h"},
			wantErrContains: "must be positive",
		},
		{
			name:            "invalid source via flag rejected",
			args:            []string{"--sources", "twitter"},
			wantErrContains: "unknown source",
		},
		{
			name:            "invalid source via config file rejected",
			configYAML:      "sources:\n  - hackernews_bogus\n",
			args:            []string{},
			wantErrContains: "unknown source",
		},
		{
			name:            "undefined flag rejected",
			args:            []string{"--bogus-flag"},
			wantErrContains: "not defined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetGlobals(t)
			args := []string{"--config", writeConfig(t, tt.configYAML)}
			args = append(args, tt.args...)
			_, err := parseFlags(args)
			if err == nil {
				t.Fatalf("expected error containing %q, got success", tt.wantErrContains)
			}
			if !strings.Contains(err.Error(), tt.wantErrContains) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrContains)
			}
		})
	}
}

// TestParseFlagsGenConfigHonorsConfigPath verifies --gen-config writes to the
// --config path when provided and that the generated comment enumerates all
// valid source IDs.
func TestParseFlagsGenConfigHonorsConfigPath(t *testing.T) {
	resetGlobals(t)
	path := filepath.Join(t.TempDir(), "custom", "my.yaml")
	cfg, err := parseFlags([]string{"--gen-config", "--config", path})
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}
	if cfg != nil {
		t.Errorf("--gen-config should return nil config, got %+v", cfg)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("generated config not written to --config path: %v", err)
	}
	for id := range techpulse.ValidSources {
		if !strings.Contains(string(data), id) {
			t.Errorf("generated config missing source %q", id)
		}
	}
}

// TestCreateConfigProviderKeepsLastGoodOnFailure verifies daemon hot-reload
// keeps the last known good config on load or file-validation failure and
// picks up good edits again afterwards.
func TestCreateConfigProviderKeepsLastGoodOnFailure(t *testing.T) {
	resetGlobals(t)
	path := writeConfig(t, "limit: 40\n")
	configPath = path

	provider := createConfigProvider(techpulse.DefaultConfig())
	if provider == nil {
		t.Fatal("expected non-nil provider when a config file exists")
	}

	if got := provider(); got.Limit != 40 {
		t.Fatalf("initial provider Limit = %d, want 40", got.Limit)
	}

	// Unloadable file: keep last good.
	if err := os.WriteFile(path, []byte("limit: [broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := provider(); got.Limit != 40 {
		t.Errorf("after unloadable file, Limit = %d, want last-good 40", got.Limit)
	}

	// Semantically invalid file: keep last good.
	if err := os.WriteFile(path, []byte("limit: 999999\nsources:\n  - bogus_source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := provider(); got.Limit != 40 {
		t.Errorf("after invalid file, Limit = %d, want last-good 40", got.Limit)
	}

	// A good edit takes effect again.
	if err := os.WriteFile(path, []byte("limit: 70\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := provider(); got.Limit != 70 {
		t.Errorf("after fixing file, Limit = %d, want 70", got.Limit)
	}
}

// TestAvailableSourcesHelpListsAllValidSources checks the --sources help text
// enumerates every ID in techpulse.ValidSources.
func TestAvailableSourcesHelpListsAllValidSources(t *testing.T) {
	help := availableSourcesHelp()
	for id := range techpulse.ValidSources {
		if !strings.Contains(help, id) {
			t.Errorf("help text %q missing source %q", help, id)
		}
	}
}

// TestSplitSources trims whitespace around comma-separated source names.
func TestSplitSources(t *testing.T) {
	got := splitSources(" rss , reddit ,")
	want := []string{"rss", "reddit"}
	if !slices.Equal(got, want) {
		t.Errorf("splitSources = %v, want %v", got, want)
	}
}
