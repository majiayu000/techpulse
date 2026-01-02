// Package techpulse provides the core TechPulse functionality.
package techpulse

import "fmt"

// Version information set at compile time using ldflags.
var (
	// Version is the semantic version number.
	Version = "1.0.0"
	// BuildDate is the date when the binary was built.
	BuildDate = "unknown"
	// GitCommit is the git commit hash.
	GitCommit = "unknown"
)

// VersionInfo returns a formatted version string.
func VersionInfo() string {
	return fmt.Sprintf("TechPulse v%s", Version)
}

// FullVersionInfo returns detailed version information.
func FullVersionInfo() string {
	return fmt.Sprintf("TechPulse v%s (built: %s, commit: %s)", Version, BuildDate, GitCommit)
}
