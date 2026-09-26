package summarizer

import "testing"

func TestDefaultSourceWeights(t *testing.T) {
	weights := DefaultSourceWeights()
	if len(weights) == 0 {
		t.Error("DefaultSourceWeights should not be empty")
	}

	// Check that known sources exist
	knownSources := []string{"hackernews", "lobsters", "github_trending", "reddit", "rss"}
	weightMap := make(map[string]float64)
	for _, w := range weights {
		weightMap[w.Name] = w.Weight
	}

	for _, source := range knownSources {
		if _, ok := weightMap[source]; !ok {
			t.Errorf("Expected source %s in default weights", source)
		}
	}
}

func TestNewSourceWeights(t *testing.T) {
	sw := NewSourceWeights()
	if sw == nil {
		t.Fatal("NewSourceWeights should not return nil")
	}
	if len(sw.weights) == 0 {
		t.Error("NewSourceWeights should have default weights")
	}
}

func TestSourceWeightsGetWeight(t *testing.T) {
	sw := NewSourceWeights()

	tests := []struct {
		source   string
		expected float64
	}{
		{"hackernews_top", 1.5},   // Prefix match
		{"hackernews", 1.5},       // Exact match
		{"HACKERNEWS", 1.5},       // Case insensitive
		{"lobsters_hottest", 1.5}, // Prefix match
		{"github_trending_daily", 1.4},
		{"reddit", 1.0},
		{"unknown_source", 1.0}, // Default neutral
	}

	for _, tt := range tests {
		got := sw.GetWeight(tt.source)
		if got != tt.expected {
			t.Errorf("GetWeight(%s) = %v, want %v", tt.source, got, tt.expected)
		}
	}
}

func TestSourceWeightsAddWeight(t *testing.T) {
	sw := NewSourceWeights()

	// Add new source
	sw.AddWeight("custom_source", 1.8)
	got := sw.GetWeight("custom_source")
	if got != 1.8 {
		t.Errorf("GetWeight(custom_source) = %v, want 1.8", got)
	}

	// Update existing source
	sw.AddWeight("hackernews", 2.0)
	got = sw.GetWeight("hackernews")
	if got != 2.0 {
		t.Errorf("GetWeight(hackernews) after update = %v, want 2.0", got)
	}
}

func TestSourceWeightsCalculateBonus(t *testing.T) {
	sw := NewSourceWeights()

	tests := []struct {
		source   string
		expected float64
	}{
		{"hackernews", 1.0},      // weight 1.5 -> bonus +1.0
		{"lobsters", 1.0},        // weight 1.5 -> bonus +1.0
		{"github_trending", 0.8}, // weight 1.4 -> bonus +0.8
		{"reddit", 0.0},          // weight 1.0 -> bonus 0.0
		{"unknown", 0.0},         // default weight 1.0 -> bonus 0.0
	}

	const epsilon = 0.001
	for _, tt := range tests {
		got := sw.CalculateBonus(tt.source)
		diff := got - tt.expected
		if diff < -epsilon || diff > epsilon {
			t.Errorf("CalculateBonus(%s) = %v, want %v", tt.source, got, tt.expected)
		}
	}
}

func TestSourceWeightsCalculateBonusClamping(t *testing.T) {
	sw := &SourceWeights{
		weights: []SourceWeight{
			{Name: "super_trusted", Weight: 3.0}, // Would be +4.0, clamped to +2.0
			{Name: "untrusted", Weight: 0.2},     // Would be -1.6, clamped to -1.0
		},
	}

	bonus := sw.CalculateBonus("super_trusted")
	if bonus != 2.0 {
		t.Errorf("Bonus for super_trusted should be clamped to 2.0, got %v", bonus)
	}

	bonus = sw.CalculateBonus("untrusted")
	if bonus != -1.0 {
		t.Errorf("Bonus for untrusted should be clamped to -1.0, got %v", bonus)
	}
}
