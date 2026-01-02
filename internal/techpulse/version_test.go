package techpulse

import (
	"strings"
	"testing"
)

func TestVersionInfo(t *testing.T) {
	info := VersionInfo()
	if !strings.HasPrefix(info, "TechPulse v") {
		t.Errorf("VersionInfo() = %q, want prefix 'TechPulse v'", info)
	}
	if !strings.Contains(info, Version) {
		t.Errorf("VersionInfo() = %q, should contain version %q", info, Version)
	}
}

func TestFullVersionInfo(t *testing.T) {
	info := FullVersionInfo()
	if !strings.HasPrefix(info, "TechPulse v") {
		t.Errorf("FullVersionInfo() = %q, want prefix 'TechPulse v'", info)
	}
	if !strings.Contains(info, "built:") {
		t.Errorf("FullVersionInfo() = %q, should contain 'built:'", info)
	}
	if !strings.Contains(info, "commit:") {
		t.Errorf("FullVersionInfo() = %q, should contain 'commit:'", info)
	}
}

func TestVersionVariables(t *testing.T) {
	// Version should not be empty
	if Version == "" {
		t.Error("Version should not be empty")
	}
	// BuildDate and GitCommit can be "unknown" by default
	if BuildDate == "" {
		t.Error("BuildDate should not be empty (can be 'unknown')")
	}
	if GitCommit == "" {
		t.Error("GitCommit should not be empty (can be 'unknown')")
	}
}
