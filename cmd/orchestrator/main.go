// Command orchestrator is the Autonomous Runner entrypoint.
// It wires config → memory → worker → progress and loops until stop conditions.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/anthropic/autonomous-runner/internal/config"
	"github.com/anthropic/autonomous-runner/internal/memory"
	"github.com/anthropic/autonomous-runner/internal/progress"
	"github.com/anthropic/autonomous-runner/internal/worker"
)

func main() {
	os.Exit(run())
}

func run() int {
	dir := flag.String("dir", ".", "project directory (memory/workspace/logs)")
	maxIterations := flag.Int("max-iterations", 100, "max iterations (0=unlimited)")
	maxCost := flag.Float64("max-cost", 50, "max cost in USD (0=unlimited)")
	maxDurationHours := flag.Float64("max-duration", 8, "max duration in hours (0=unlimited)")
	flag.Parse()

	projectDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve project dir: %v\n", err)
		return 1
	}

	cfgPath := filepath.Join(projectDir, "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}

	// Only apply CLI overrides for flags that were explicitly supplied so
	// config.yaml limits are preserved when the binary is invoked bare.
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "max-iterations":
			cfg.MaxIterations = *maxIterations
		case "max-cost":
			cfg.MaxCostUSD = *maxCost
		case "max-duration":
			if *maxDurationHours <= 0 {
				cfg.MaxDuration = 0
			} else {
				cfg.MaxDuration = time.Duration(*maxDurationHours * float64(time.Hour))
			}
		}
	})
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "validate config: %v\n", err)
		return 1
	}

	memoryDir := resolvePath(projectDir, cfg.MemoryDir)
	workspaceDir := resolvePath(projectDir, cfg.WorkspaceDir)
	logDir := resolvePath(projectDir, cfg.LogDir)

	for _, d := range []string{workspaceDir, logDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "create dir %s: %v\n", d, err)
			return 1
		}
	}

	logPath := filepath.Join(logDir, fmt.Sprintf("orchestrator_%s.log", time.Now().Format("20060102_150405")))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open log file: %v\n", err)
		return 1
	}
	defer logFile.Close()
	out := io.MultiWriter(os.Stdout, logFile)
	printf := func(format string, args ...any) {
		fmt.Fprintf(out, format, args...)
	}

	mem, err := memory.NewManager(memoryDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init memory: %v\n", err)
		return 1
	}

	hashDet, err := progress.NewHashDetector(mem)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init hash detector: %v\n", err)
		return 1
	}
	detectors := []progress.Detector{hashDet}
	if cfg.UseGitDetection {
		gitDet, err := progress.NewGitDetector(workspaceDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: git detector disabled: %v\n", err)
		} else {
			// Operational logs must not reset consecutive_no_progress.
			gitDet.IgnorePaths(logPath)
			detectors = append(detectors, gitDet)
		}
	}
	multi := progress.NewMultiDetector(detectors...)

	runner := worker.NewRunner(mem, workspaceDir, cfg.WorkerTimeout)

	// Graceful shutdown: signal stops further iterations but does not cancel
	// the active worker (README: Ctrl+C finishes the current task first).
	var stopRequested atomic.Bool
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		stopRequested.Store(true)
		printf("\nsignal received — finishing current iteration, then stopping\n")
	}()

	printf("Autonomous Runner starting\n")
	printf("  project:     %s\n", projectDir)
	printf("  memory:      %s\n", memoryDir)
	printf("  workspace:   %s\n", workspaceDir)
	printf("  log file:    %s\n", logPath)
	printf("  max iter:    %s\n", limitInt(cfg.MaxIterations))
	printf("  max cost:    %s\n", limitFloat(cfg.MaxCostUSD))
	printf("  max duration:%s\n", limitDuration(cfg.MaxDuration))
	printf("\n")

	start := time.Now()
	var (
		iterations      int
		totalCost       float64
		totalTokens     int
		successCount    int
		failCount       int
		noProgressCount int
		stopReason      = "unknown"
	)

	for {
		if stopRequested.Load() {
			stopReason = "signal"
			break
		}
		if cfg.MaxIterations > 0 && iterations >= cfg.MaxIterations {
			stopReason = "max_iterations"
			break
		}
		if cfg.MaxCostUSD > 0 && totalCost >= cfg.MaxCostUSD {
			stopReason = "max_cost"
			break
		}
		if cfg.MaxDuration > 0 && time.Since(start) >= cfg.MaxDuration {
			stopReason = "max_duration"
			break
		}
		if cfg.StopWhenEmpty {
			hasPending, err := mem.HasPendingTasks()
			if err != nil {
				fmt.Fprintf(os.Stderr, "check pending tasks: %v\n", err)
				stopReason = "error"
				break
			}
			if !hasPending {
				stopReason = "stop_when_empty"
				break
			}
		}

		// Enforce MaxDuration on the active worker; do not tie worker lifetime
		// to the signal channel so graceful shutdown can finish the iteration.
		runCtx := context.Background()
		var cancel context.CancelFunc
		if cfg.MaxDuration > 0 {
			remaining := cfg.MaxDuration - time.Since(start)
			if remaining <= 0 {
				stopReason = "max_duration"
				break
			}
			runCtx, cancel = context.WithTimeout(context.Background(), remaining)
		}

		iterations++
		printf("--- iteration %d ---\n", iterations)

		if err := multi.Reset(); err != nil {
			fmt.Fprintf(os.Stderr, "reset detectors: %v\n", err)
		}

		result := runner.Run(runCtx)
		if cancel != nil {
			cancel()
		}
		totalCost += result.Cost
		totalTokens += result.Tokens
		if result.Success {
			successCount++
			printf("worker ok  cost=$%.4f tokens=%d duration=%s\n",
				result.Cost, result.Tokens, result.Duration.Round(time.Millisecond))
		} else {
			failCount++
			errMsg := "unknown"
			if result.Error != nil {
				errMsg = result.Error.Error()
			}
			printf("worker fail type=%s err=%s\n", result.ErrorType, errMsg)
		}

		prog, err := multi.Detect()
		if err != nil {
			fmt.Fprintf(os.Stderr, "progress detect: %v\n", err)
		} else if prog != nil && prog.HasProgress {
			noProgressCount = 0
			printf("progress: yes (%s) %s\n", prog.Source, prog.Details)
		} else {
			noProgressCount++
			printf("progress: no (%d consecutive)\n", noProgressCount)
		}

		if cfg.ConsecutiveNoProgress > 0 && noProgressCount >= cfg.ConsecutiveNoProgress {
			stopReason = "consecutive_no_progress"
			break
		}

		if cfg.MaxCostUSD > 0 && totalCost >= cfg.MaxCostUSD {
			stopReason = "max_cost"
			break
		}
		if stopRequested.Load() {
			stopReason = "signal"
			break
		}
		// Skip cooldown when another iteration is not allowed.
		if cfg.MaxIterations > 0 && iterations >= cfg.MaxIterations {
			stopReason = "max_iterations"
			break
		}
		if cfg.MaxDuration > 0 && time.Since(start) >= cfg.MaxDuration {
			stopReason = "max_duration"
			break
		}

		if cfg.CooldownDuration > 0 {
			cooldown := cfg.CooldownDuration
			durationLimited := false
			if cfg.MaxDuration > 0 {
				remaining := cfg.MaxDuration - time.Since(start)
				if remaining <= 0 {
					stopReason = "max_duration"
					break
				}
				if cooldown > remaining {
					cooldown = remaining
					durationLimited = true
				}
			}
			printf("cooldown %s...\n", cooldown)
			timer := time.NewTicker(100 * time.Millisecond)
			deadline := time.Now().Add(cooldown)
			interrupted := false
			for time.Now().Before(deadline) {
				if stopRequested.Load() {
					interrupted = true
					break
				}
				if cfg.MaxDuration > 0 && time.Since(start) >= cfg.MaxDuration {
					durationLimited = true
					break
				}
				<-timer.C
			}
			timer.Stop()
			if interrupted {
				stopReason = "signal"
				break
			}
			if durationLimited || (cfg.MaxDuration > 0 && time.Since(start) >= cfg.MaxDuration) {
				stopReason = "max_duration"
				break
			}
		}
	}

	elapsed := time.Since(start).Round(time.Second)
	printf("\n")
	printf("========== run summary ==========\n")
	printf("stop reason:  %s\n", stopReason)
	printf("iterations:   %d (ok=%d fail=%d)\n", iterations, successCount, failCount)
	printf("total cost:   $%.4f\n", totalCost)
	printf("total tokens: %d\n", totalTokens)
	printf("elapsed:      %s\n", elapsed)
	printf("=================================\n")

	if stopReason == "error" {
		return 1
	}
	// Nonzero when the run produced only worker failures (no successful work).
	if successCount == 0 && failCount > 0 {
		return 1
	}
	return 0
}

func resolvePath(projectDir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(projectDir, p)
}

func limitInt(n int) string {
	if n <= 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d", n)
}

func limitFloat(n float64) string {
	if n <= 0 {
		return "unlimited"
	}
	return fmt.Sprintf("$%.2f", n)
}

func limitDuration(d time.Duration) string {
	if d <= 0 {
		return "unlimited"
	}
	return d.String()
}
