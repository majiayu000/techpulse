// TechPulse - AI/Tech news collector and aggregator
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/anthropic/autonomous-runner/internal/logger"
	"github.com/anthropic/autonomous-runner/internal/techpulse"
)

var log = logger.Default()

var (
	daemon, quiet, showProgress, enableSummary, showVersion, validateOnly bool
	interval                                                               time.Duration
	configPath                                                             string
)

func main() {
	cfg := parseFlags()
	if cfg == nil {
		return
	}
	if quiet {
		logger.SetDefault(logger.NewNopLogger())
		log = logger.Default()
	} else {
		printBanner()
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go handleSignals(cancel)
	if daemon {
		runDaemon(ctx, *cfg)
	} else {
		runOnce(ctx, *cfg)
	}
}

func runOnce(ctx context.Context, cfg techpulse.Config) {
	tp := techpulse.NewWithOptions(cfg)
	tp.SetShowProgress(showProgress && !quiet)
	if err := tp.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if !quiet {
		fmt.Printf("\nReport available at: %s/DIGEST.md\n", cfg.Output)
	}
}

func runDaemon(ctx context.Context, cfg techpulse.Config) {
	log.Info("Starting daemon mode", logger.F("interval", interval.String()))
	if !quiet {
		fmt.Printf("Daemon mode: collecting every %s\nPress Ctrl+C to stop\n", interval)
	}
	daemonCfg := techpulse.DaemonConfig{
		Interval: interval, ConfigProvider: createConfigProvider(cfg),
		OnTick: func() {
			if !quiet {
				fmt.Printf("\n[%s] Starting collection...\n", time.Now().Format("2006-01-02 15:04:05"))
			}
		},
		OnDone: func() {
			if !quiet {
				fmt.Printf("Report saved. Next: %s\n", time.Now().Add(interval).Format("15:04:05"))
			}
		},
		OnStop: func() { if !quiet { fmt.Println("\nDaemon stopped") } },
	}
	d := techpulse.NewDaemon(cfg, daemonCfg)
	d.SetLogger(log)
	d.Run(ctx)
}

func createConfigProvider(baseCfg techpulse.Config) func() techpulse.Config {
	if configPath == "" { configPath = techpulse.FindConfigFile() }
	if configPath == "" { return nil }
	return func() techpulse.Config {
		fileCfg, err := techpulse.LoadConfigFile(configPath)
		if err != nil { return baseCfg }
		return techpulse.MergeWithConfig(baseCfg, fileCfg)
	}
}

func parseFlags() *techpulse.Config {
	cfg := techpulse.DefaultConfig()
	var sources string
	var listSources bool
	var genConfig bool
	var intervalStr string

	flag.IntVar(&cfg.Limit, "limit", cfg.Limit, "Maximum articles per source")
	flag.StringVar(&sources, "sources", "", "Comma-separated sources (e.g., hackernews,rss)")
	flag.StringVar(&cfg.Output, "output", cfg.Output, "Output directory for reports")
	flag.IntVar(&cfg.Timeout, "timeout", cfg.Timeout, "Request timeout in seconds")
	flag.BoolVar(&listSources, "list-sources", false, "List available data sources")
	flag.StringVar(&configPath, "config", "", "Path to config file (default: auto-detect)")
	flag.BoolVar(&genConfig, "gen-config", false, "Generate example config file")
	flag.BoolVar(&daemon, "daemon", false, "Run as daemon (continuous collection)")
	flag.StringVar(&intervalStr, "interval", "24h", "Collection interval in daemon mode (e.g., 1h, 24h)")
	flag.BoolVar(&quiet, "quiet", false, "Suppress all output except errors")
	flag.BoolVar(&quiet, "q", false, "Suppress all output except errors (shorthand)")
	flag.BoolVar(&showProgress, "progress", false, "Show collection progress bar")
	flag.BoolVar(&enableSummary, "summary", false, "Extract content summaries from articles")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (shorthand)")
	flag.BoolVar(&validateOnly, "validate", false, "Validate config file and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(techpulse.FullVersionInfo())
		return nil
	}

	var err error
	if interval, err = time.ParseDuration(intervalStr); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid interval: %v\n", err)
		os.Exit(1)
	}

	if genConfig {
		if err := techpulse.GenerateExampleConfig("techpulse.yaml"); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Example config generated: techpulse.yaml")
		return nil
	}
	if listSources {
		fmt.Println("Available sources:")
		for _, s := range techpulse.New().AvailableSources() {
			fmt.Printf("  - %s\n", s)
		}
		return nil
	}

	cfg = loadConfigFile(cfg)
	if sources != "" {
		cfg.Sources = strings.Split(sources, ",")
	}
	cfg.EnableSummary = enableSummary

	if validateOnly {
		if errs := techpulse.ValidateConfig(cfg); len(errs) > 0 {
			fmt.Fprintf(os.Stderr, "Config validation failed:\n")
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "  - %s\n", e)
			}
			os.Exit(1)
		}
		fmt.Println("Config is valid!")
		return nil
	}
	return &cfg
}

func loadConfigFile(cfg techpulse.Config) techpulse.Config {
	if configPath == "" {
		configPath = techpulse.FindConfigFile()
	}
	if configPath == "" {
		return cfg
	}
	fileCfg, err := techpulse.LoadConfigFile(configPath)
	if err != nil {
		log.Warn("Could not load config", logger.F("error", err))
		return cfg
	}
	if errs := techpulse.ValidateFileConfig(fileCfg); len(errs) > 0 {
		log.Warn("Config issues", logger.F("errors", errs.Error()))
	}
	log.Info("Using config", logger.F("path", configPath))
	return techpulse.MergeWithConfig(cfg, fileCfg)
}

func printBanner() {
	fmt.Printf("╔══════════════════════════════════════╗\n║         %-28s ║\n║   AI/Tech News Collector             ║\n╚══════════════════════════════════════╝\n\n", techpulse.VersionInfo())
}

func handleSignals(cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	<-c
	log.Warn("Interrupted, shutting down...")
	cancel()
}
