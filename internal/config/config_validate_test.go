package config

import (
	"math"
	"testing"
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
