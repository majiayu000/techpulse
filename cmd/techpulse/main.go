// TechPulse - AI/Tech news collector and aggregator
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/majiayu000/techpulse/internal/logger"
	"github.com/majiayu000/techpulse/internal/techpulse"
)

var log = logger.Default()

// Flag targets. These stay package-level because the run loops
// (runOnce/runDaemon) read them after parsing.
var (
	daemon, quiet, showProgress, showVersion, validateOnly bool
	interval                                               time.Duration
	configPath                                             string
)

// cliOpts records the Config-affecting flag values and which flags were
// actually passed on the command line. Populated by parseFlags and consulted
// again by createConfigProvider so daemon hot reloads apply the same
// precedence rules as startup.
var cliOpts cliFlags

// cliFlags holds Config-affecting flag values plus the set of flags that were
// explicitly present in argv.
type cliFlags struct {
	limit         int
	output        string
	timeout       int
	retentionDays int
	sources       string
	summary       bool
	set           map[string]bool // flag names explicitly passed by the user
}

// applyTo layers explicitly-passed CLI flags over cfg.
//
// Precedence (highest to lowest):
//  1. CLI flags explicitly passed on the command line
//  2. Values from the config file (--config or auto-detected)
//  3. Built-in defaults
//
// Only flags present in argv take part in step 1; a flag left at its default
// never masquerades as a user choice and cannot clobber the config file.
func (f cliFlags) applyTo(cfg techpulse.Config) techpulse.Config {
	if f.set["limit"] {
		cfg.Limit = f.limit
	}
	if f.set["output"] {
		cfg.Output = f.output
	}
	if f.set["timeout"] {
		cfg.Timeout = f.timeout
	}
	if f.set["retention-days"] {
		cfg.RetentionDays = f.retentionDays
	}
	if f.set["sources"] {
		// Explicit --sources= (empty) clears a file-configured subset and
		// restores empty-means-all behavior.
		cfg.Sources = splitSources(f.sources)
	}
	if f.set["summary"] {
		cfg.EnableSummary = f.summary
	}
	return cfg
}

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if cfg == nil {
		// An informational action (--version, --gen-config, --list-sources,
		// --validate, --help) completed successfully.
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
		OnStop: func() {
			if !quiet {
				fmt.Println("\nDaemon stopped")
			}
		},
	}
	d := techpulse.NewDaemon(cfg, daemonCfg)
	d.SetLogger(log)
	if _, err := d.Run(ctx); err != nil &&
		!errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// createConfigProvider returns a provider that re-reads the config file before
// each daemon tick (hot-reload). A file that fails to load, or whose effective
// configuration (defaults + file + explicit CLI flags) fails runtime
// validation, is logged at error level and the last known good configuration
// is kept. Validation runs after CLI overrides — matching startup — so a file
// value that is invalid on its own but overridden by a valid CLI flag (e.g.
// limit: 999 with --limit 10) still reloads successfully.
func createConfigProvider(baseCfg techpulse.Config) func() techpulse.Config {
	path := configPath
	if path == "" {
		path = techpulse.FindConfigFile()
	}
	if path == "" {
		return nil
	}
	lastGood := baseCfg
	return func() techpulse.Config {
		fileCfg, err := techpulse.LoadConfigFile(path)
		if err != nil {
			log.Error("Config reload failed, keeping last good config",
				logger.F("path", path), logger.F("error", err))
			return lastGood
		}
		cfg := cliOpts.applyTo(techpulse.MergeWithConfig(techpulse.DefaultConfig(), fileCfg))
		if errs := techpulse.ValidateConfig(cfg); len(errs) > 0 {
			log.Error("Reloaded config failed validation, keeping last good config",
				logger.F("path", path), logger.F("errors", errs.Error()))
			return lastGood
		}
		lastGood = cfg
		return lastGood
	}
}

// parseFlags parses args into a validated runtime configuration.
//
// It returns:
//   - (&cfg, nil): run with cfg
//   - (nil, nil): an informational action (--version, --gen-config,
//     --list-sources, --validate, --help) already ran successfully
//   - (nil, err): startup must fail; the caller should report err and exit non-zero
func parseFlags(args []string) (*techpulse.Config, error) {
	defaults := techpulse.DefaultConfig()
	var opts cliFlags
	var listSources, genConfig bool
	var intervalStr string

	fs := flag.NewFlagSet("techpulse", flag.ContinueOnError)
	fs.IntVar(&opts.limit, "limit", defaults.Limit, "Maximum articles per source")
	fs.StringVar(&opts.sources, "sources", "", "Comma-separated sources (available: "+availableSourcesHelp()+")")
	fs.StringVar(&opts.output, "output", defaults.Output, "Output directory for reports")
	fs.IntVar(&opts.timeout, "timeout", defaults.Timeout, "Per-request HTTP timeout in seconds")
	fs.IntVar(&opts.retentionDays, "retention-days", defaults.RetentionDays, "Archive retention days (0 disables cleanup)")
	fs.BoolVar(&listSources, "list-sources", false, "List available data sources")
	fs.StringVar(&configPath, "config", "", "Path to config file (default: auto-detect)")
	fs.BoolVar(&genConfig, "gen-config", false, "Generate example config file (honors --config path)")
	fs.BoolVar(&daemon, "daemon", false, "Run as daemon (continuous collection)")
	fs.StringVar(&intervalStr, "interval", "24h", "Collection interval in daemon mode (e.g., 1h, 24h)")
	fs.BoolVar(&quiet, "quiet", false, "Suppress all output except errors")
	fs.BoolVar(&quiet, "q", false, "Suppress all output except errors (shorthand)")
	fs.BoolVar(&showProgress, "progress", false, "Show collection progress bar")
	fs.BoolVar(&opts.summary, "summary", false, "Extract content summaries from articles")
	fs.BoolVar(&showVersion, "version", false, "Show version information")
	fs.BoolVar(&showVersion, "v", false, "Show version information (shorthand)")
	fs.BoolVar(&validateOnly, "validate", false, "Validate config and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, nil // usage was printed by the flag package
		}
		return nil, fmt.Errorf("parse flags: %w", err)
	}

	// Record which flags the user actually passed so merge can tell explicit
	// choices apart from defaults (see cliFlags.applyTo).
	opts.set = make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { opts.set[f.Name] = true })
	cliOpts = opts

	if showVersion {
		fmt.Println(techpulse.FullVersionInfo())
		return nil, nil
	}

	var err error
	if interval, err = time.ParseDuration(intervalStr); err != nil {
		return nil, fmt.Errorf("invalid interval %q: %w", intervalStr, err)
	}
	if interval <= 0 {
		return nil, fmt.Errorf("invalid interval %q: must be positive", intervalStr)
	}

	if genConfig {
		path := configPath
		if path == "" {
			path = "techpulse.yaml"
		}
		if err := techpulse.GenerateExampleConfig(path); err != nil {
			return nil, fmt.Errorf("generate example config: %w", err)
		}
		fmt.Println("Example config generated:", path)
		return nil, nil
	}

	if listSources {
		fmt.Println("Available sources:")
		for _, s := range techpulse.New().AvailableSources() {
			fmt.Printf("  - %s\n", s)
		}
		return nil, nil
	}

	fileCfg, err := loadFileConfig()
	if err != nil {
		return nil, err
	}

	// Precedence: explicit CLI flags > config file > defaults. The file is
	// merged over the defaults first; then only user-passed flags override it.
	cfg := techpulse.MergeWithConfig(defaults, fileCfg)
	cfg = opts.applyTo(cfg)

	if errs := techpulse.ValidateConfig(cfg); len(errs) > 0 {
		return nil, validationError(errs)
	}

	if validateOnly {
		// File-level problems that runtime validation does not cover
		// (e.g. malformed rss_feeds entries) must still fail --validate.
		if errs := techpulse.ValidateFileConfig(fileCfg); len(errs) > 0 {
			return nil, validationError(errs)
		}
		fmt.Println("Config is valid!")
		return nil, nil
	}

	return &cfg, nil
}

// loadFileConfig loads the YAML config given via --config or auto-detected,
// returning (nil, nil) when no config file is in play. A resolved config file
// that cannot be read or parsed is a hard error: TechPulse fails closed rather
// than silently running with different settings than the user wrote down.
func loadFileConfig() (*techpulse.FileConfig, error) {
	if configPath == "" {
		configPath = techpulse.FindConfigFile()
	}
	if configPath == "" {
		return nil, nil
	}
	fileCfg, err := techpulse.LoadConfigFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config file %q: %w", configPath, err)
	}
	if errs := techpulse.ValidateFileConfig(fileCfg); len(errs) > 0 {
		log.Warn("Config file has issues", logger.F("path", configPath), logger.F("errors", errs.Error()))
	}
	log.Info("Using config", logger.F("path", configPath))
	return fileCfg, nil
}

// validationError renders ValidationErrors as a single readable error value.
func validationError(errs techpulse.ValidationErrors) error {
	var b strings.Builder
	b.WriteString("configuration validation failed:")
	for _, e := range errs {
		fmt.Fprintf(&b, "\n  - %s", e)
	}
	return errors.New(b.String())
}

// availableSourcesHelp returns all valid source IDs, sorted, for help text.
func availableSourcesHelp() string {
	ids := make([]string, 0, len(techpulse.ValidSources))
	for id := range techpulse.ValidSources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return strings.Join(ids, ", ")
}

// splitSources splits a comma-separated source list, trimming whitespace and
// dropping empty entries so "rss, reddit" yields clean source IDs.
func splitSources(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
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
