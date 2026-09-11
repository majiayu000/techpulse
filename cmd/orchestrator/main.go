// Command orchestrator is the Autonomous Runner entrypoint.
// It wires config → memory → worker → progress and loops until stop conditions.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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

	// CLI overrides (run.sh always passes these; 0 means unlimited)
	cfg.MaxIterations = *maxIterations
	cfg.MaxCostUSD = *maxCost
	if *maxDurationHours <= 0 {
		cfg.MaxDuration = 0
	} else {
		cfg.MaxDuration = time.Duration(*maxDurationHours * float64(time.Hour))
	}
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
			detectors = append(detectors, gitDet)
		}
	}
	multi := progress.NewMultiDetector(detectors...)

	runner := worker.NewRunner(mem, workspaceDir, cfg.WorkerTimeout)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("Autonomous Runner starting\n")
	fmt.Printf("  project:     %s\n", projectDir)
	fmt.Printf("  memory:      %s\n", memoryDir)
	fmt.Printf("  workspace:   %s\n", workspaceDir)
	fmt.Printf("  max iter:    %s\n", limitInt(cfg.MaxIterations))
	fmt.Printf("  max cost:    %s\n", limitFloat(cfg.MaxCostUSD))
	fmt.Printf("  max duration:%s\n", limitDuration(cfg.MaxDuration))
	fmt.Println()

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
		if ctx.Err() != nil {
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

		iterations++
		fmt.Printf("--- iteration %d ---\n", iterations)

		if err := multi.Reset(); err != nil {
			fmt.Fprintf(os.Stderr, "reset detectors: %v\n", err)
		}

		result := runner.Run(ctx)
		totalCost += result.Cost
		totalTokens += result.Tokens
		if result.Success {
			successCount++
			fmt.Printf("worker ok  cost=$%.4f tokens=%d duration=%s\n",
				result.Cost, result.Tokens, result.Duration.Round(time.Millisecond))
		} else {
			failCount++
			errMsg := "unknown"
			if result.Error != nil {
				errMsg = result.Error.Error()
			}
			fmt.Printf("worker fail type=%s err=%s\n", result.ErrorType, errMsg)
		}

		prog, err := multi.Detect()
		if err != nil {
			fmt.Fprintf(os.Stderr, "progress detect: %v\n", err)
		} else if prog != nil && prog.HasProgress {
			noProgressCount = 0
			fmt.Printf("progress: yes (%s) %s\n", prog.Source, prog.Details)
		} else {
			noProgressCount++
			fmt.Printf("progress: no (%d consecutive)\n", noProgressCount)
		}

		if cfg.ConsecutiveNoProgress > 0 && noProgressCount >= cfg.ConsecutiveNoProgress {
			stopReason = "consecutive_no_progress"
			break
		}

		if cfg.MaxCostUSD > 0 && totalCost >= cfg.MaxCostUSD {
			stopReason = "max_cost"
			break
		}
		if ctx.Err() != nil {
			stopReason = "signal"
			break
		}

		if cfg.CooldownDuration > 0 {
			fmt.Printf("cooldown %s...\n", cfg.CooldownDuration)
			timer := time.NewTimer(cfg.CooldownDuration)
			select {
			case <-ctx.Done():
				timer.Stop()
				stopReason = "signal"
				goto done
			case <-timer.C:
			}
		}
	}

done:
	elapsed := time.Since(start).Round(time.Second)
	fmt.Println()
	fmt.Println("========== run summary ==========")
	fmt.Printf("stop reason:  %s\n", stopReason)
	fmt.Printf("iterations:   %d (ok=%d fail=%d)\n", iterations, successCount, failCount)
	fmt.Printf("total cost:   $%.4f\n", totalCost)
	fmt.Printf("total tokens: %d\n", totalTokens)
	fmt.Printf("elapsed:      %s\n", elapsed)
	fmt.Println("=================================")

	if stopReason == "error" {
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
