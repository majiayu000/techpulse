package config

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateRejectsNonFiniteMaxCost(t *testing.T) {
	for _, cost := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		cfg := Default()
		cfg.MaxCostUSD = cost
		if err := cfg.Validate(); err == nil {
			t.Fatalf("Validate() should reject max_cost_usd=%v", cost)
		}
	}
}

func TestValidateAcceptsFiniteMaxCost(t *testing.T) {
	cfg := Default()
	cfg.MaxCostUSD = 12.5
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestLoadValidateWorkerTimeout(t *testing.T) {
	for _, tt := range []struct {
		name    string
		yaml    string
		want    time.Duration
		wantErr bool
	}{
		{"omitted", "", 30 * time.Minute, false},
		{"negative", "worker_timeout: -1s\n", -time.Second, true},
		{"zero", "worker_timeout: 0s\n", 0, true},
		{"ten_seconds", "worker_timeout: 10s\n", 10 * time.Second, true},
		{"thirty_seconds", "worker_timeout: 30s\n", 30 * time.Second, true},
		{"below_minimum", "worker_timeout: 59.999999999s\n", time.Minute - time.Nanosecond, true},
		{"minimum", "worker_timeout: 1m\n", time.Minute, false},
		{"above_minimum", "worker_timeout: 1m0.000000001s\n", time.Minute + time.Nanosecond, false},
		{"default_explicit", "worker_timeout: 30m\n", 30 * time.Minute, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tt.yaml), 0644); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
			if cfg.WorkerTimeout != tt.want {
				t.Fatalf("Load() worker timeout = %v, want %v", cfg.WorkerTimeout, tt.want)
			}

			err = cfg.Validate()
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "worker_timeout") {
					t.Errorf("Validate() error = %v, want worker_timeout validation error", err)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
			if cfg.WorkerTimeout != tt.want {
				t.Errorf("Validate() changed worker timeout to %v, want %v", cfg.WorkerTimeout, tt.want)
			}
		})
	}
}
