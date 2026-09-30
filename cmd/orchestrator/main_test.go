package main

import (
	"errors"
	"flag"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMaxDurationCLI(t *testing.T) {
	upper_bound := float64(math.MaxInt64) / float64(time.Hour)
	largest_hours := math.Nextafter(upper_bound, 0)
	largest_duration := time.Duration(largest_hours * float64(time.Hour))
	for _, tt := range []struct {
		name      string
		args      []string
		want      string
		want_exit int
	}{
		{"NaN", []string{"--max-duration=NaN"}, "invalid --max-duration:", 1},
		{"positive infinity", []string{"--max-duration=+Inf"}, "invalid --max-duration:", 1},
		{"negative infinity", []string{"--max-duration=-Inf"}, "invalid --max-duration:", 1},
		{"positive overflow", []string{"--max-duration=1e100"}, "invalid --max-duration:", 1},
		{"negative overflow", []string{"--max-duration=-1e100"}, "invalid --max-duration:", 1},
		{"upper boundary", []string{"--max-duration=" + strconv.FormatFloat(upper_bound, 'g', -1, 64)}, "invalid --max-duration:", 1},
		{"largest finite hours", []string{"--max-duration=" + strconv.FormatFloat(largest_hours, 'g', -1, 64)}, "max duration:" + largest_duration.String(), 0},
		{"fractional hours", []string{"--max-duration=0.5"}, "max duration:30m0s", 0},
		{"zero unlimited", []string{"--max-duration=0"}, "max duration:unlimited", 0},
		{"negative unlimited", []string{"--max-duration=-1"}, "max duration:unlimited", 0},
		{"config preserved", nil, "max duration:2h0m0s", 0},
		{"cost error preserved", []string{"--max-cost=NaN", "--max-duration=NaN"}, "invalid --max-cost: must be a finite number", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("max_duration: 2h\nuse_git_detection: false\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(dir, "memory"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "memory", "TASKS.md"), []byte("# No pending tasks\n"), 0600); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"-test.run=^TestOrchestratorCLIHelper$", "--", "--dir", dir}, tt.args...)
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "TECHPULSE_TEST_ORCHESTRATOR_CLI=1")
			output, err := cmd.CombinedOutput()
			exit_code := 0
			if err != nil {
				var exit_err *exec.ExitError
				if !errors.As(err, &exit_err) {
					t.Fatal(err)
				}
				exit_code = exit_err.ExitCode()
			}
			if exit_code != tt.want_exit || !strings.Contains(string(output), tt.want) {
				t.Fatalf("exit=%d, want %d; output=%s; want %q", exit_code, tt.want_exit, output, tt.want)
			}
			if tt.want_exit != 0 {
				if _, err := os.Stat(filepath.Join(dir, "workspace")); !os.IsNotExist(err) {
					t.Fatalf("invalid flags initialized workspace: %v", err)
				}
			}
		})
	}
}

func TestOrchestratorCLIHelper(t *testing.T) {
	if os.Getenv("TECHPULSE_TEST_ORCHESTRATOR_CLI") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{os.Args[0]}, os.Args[i+1:]...)
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			os.Exit(run())
		}
	}
	t.Fatal("missing CLI arguments")
}
