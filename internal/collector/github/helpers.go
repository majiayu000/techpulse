package github

import (
	"strings"
)

// sanitizeID creates a safe ID from repository name.
func sanitizeID(name string) string {
	return strings.ReplaceAll(name, "/", "_")
}

// extractOwner extracts the owner from "owner/repo" format.
func extractOwner(name string) string {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// buildTags creates tags from repository metadata.
func buildTags(repo Repository) []string {
	tags := make([]string, 0, 3)

	if repo.Language != "" {
		tags = append(tags, repo.Language)
	}

	// Add "trending" tag
	tags = append(tags, "trending")

	// Add topic-based tags if present
	tags = append(tags, repo.Topics...)

	return tags
}
