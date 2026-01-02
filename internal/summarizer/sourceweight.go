package summarizer

import "strings"

// SourceWeight defines the weight configuration for a source.
type SourceWeight struct {
	Name   string  // Source name pattern (supports prefix matching)
	Weight float64 // Weight multiplier (1.0 = neutral, > 1.0 = boost, < 1.0 = penalty)
}

// SourceWeights manages source weight configurations.
type SourceWeights struct {
	weights []SourceWeight
}

// NewSourceWeights creates a new SourceWeights with default configuration.
func NewSourceWeights() *SourceWeights {
	return &SourceWeights{
		weights: DefaultSourceWeights(),
	}
}

// GetWeight returns the weight for a given source.
// Returns 1.0 (neutral) if no matching weight is found.
func (sw *SourceWeights) GetWeight(source string) float64 {
	source = strings.ToLower(source)
	for _, w := range sw.weights {
		pattern := strings.ToLower(w.Name)
		if strings.HasPrefix(source, pattern) || source == pattern {
			return w.Weight
		}
	}
	return 1.0 // Default neutral weight
}

// AddWeight adds or updates a source weight.
func (sw *SourceWeights) AddWeight(name string, weight float64) {
	for i, w := range sw.weights {
		if strings.EqualFold(w.Name, name) {
			sw.weights[i].Weight = weight
			return
		}
	}
	sw.weights = append(sw.weights, SourceWeight{Name: name, Weight: weight})
}

// DefaultSourceWeights returns the default source weight configuration.
// Higher weights indicate more trusted/authoritative sources.
func DefaultSourceWeights() []SourceWeight {
	return []SourceWeight{
		// Premier sources (1.5x weight)
		{Name: "hackernews", Weight: 1.5},
		{Name: "lobsters", Weight: 1.5},

		// High-quality tech news (1.3x weight)
		{Name: "techcrunch", Weight: 1.3},
		{Name: "arstechnica", Weight: 1.3},
		{Name: "theverge", Weight: 1.2},
		{Name: "wired", Weight: 1.3},
		{Name: "mit technology review", Weight: 1.4},

		// GitHub (repos are usually high quality)
		{Name: "github_trending", Weight: 1.4},

		// Reddit (varies by subreddit, average quality)
		{Name: "reddit", Weight: 1.0},

		// RSS feeds (default weight)
		{Name: "rss", Weight: 1.0},
	}
}

// CalculateBonus returns the score bonus based on source weight.
// Returns a value between -1.0 and +2.0 to add to the base importance.
func (sw *SourceWeights) CalculateBonus(source string) float64 {
	weight := sw.GetWeight(source)

	// Convert weight to bonus:
	// weight 1.5 -> +1.0 bonus
	// weight 1.0 -> +0.0 bonus
	// weight 0.5 -> -1.0 bonus
	bonus := (weight - 1.0) * 2.0

	// Clamp bonus between -1.0 and +2.0
	if bonus > 2.0 {
		bonus = 2.0
	}
	if bonus < -1.0 {
		bonus = -1.0
	}

	return bonus
}
