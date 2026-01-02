// Package github provides a collector for GitHub Trending repositories.
package github

// Repository represents a GitHub trending repository.
type Repository struct {
	Name        string   // Full name (owner/repo)
	URL         string   // GitHub URL
	Description string   // Repository description
	Language    string   // Primary programming language
	Stars       int      // Total star count
	StarsToday  int      // Stars gained today
	Forks       int      // Fork count
	Topics      []string // Repository topics
}

// ValidPeriods lists valid trending time periods.
var ValidPeriods = map[string]bool{
	"daily":   true,
	"weekly":  true,
	"monthly": true,
}

// ValidLanguages lists commonly used programming languages.
var ValidLanguages = map[string]bool{
	"":           true, // All languages
	"go":         true,
	"rust":       true,
	"python":     true,
	"javascript": true,
	"typescript": true,
	"java":       true,
	"c++":        true,
	"c":          true,
}

// DefaultPeriod is the default trending period.
const DefaultPeriod = "daily"

// DefaultLanguage means all languages (empty string).
const DefaultLanguage = ""
