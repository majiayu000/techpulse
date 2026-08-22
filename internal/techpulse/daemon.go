// Package techpulse daemon mode support.
package techpulse

import (
	"context"
	"fmt"
	"time"

	"github.com/majiayu000/techpulse/internal/logger"
)

// Retry defaults for a failed initial collection. Without them a startup
// failure (e.g. transient network outage) would idle the daemon for a full
// interval (24h by default) before trying again.
const (
	// DefaultInitialRetryDelay is the delay before the first retry attempt.
	DefaultInitialRetryDelay = 30 * time.Second
	// DefaultInitialRetryAttempts is the number of retries after the initial
	// collection fails. With the default exponential backoff this keeps the
	// total retry window in the low single-digit minutes.
	DefaultInitialRetryAttempts = 3
	// MaxInitialRetryDelay caps the exponential backoff between retries.
	MaxInitialRetryDelay = 5 * time.Minute
)

// DaemonConfig contains configuration for daemon mode.
type DaemonConfig struct {
	Interval         time.Duration           // Collection interval (must be > 0)
	ConfigProvider   func() Config           // Returns current config (for hot-reload)
	TechPulseFactory func(Config) *TechPulse // Custom factory (for testing)
	OnStart          func()                  // Called when daemon starts
	OnTick           func()                  // Called before each collection
	OnDone           func()                  // Called after each collection
	OnStop           func()                  // Called when daemon stops
	OnConfigReload   func(old, new Config)   // Called when config is reloaded

	// InitialRetryDelay is the base delay for retries of a failed initial
	// collection; it doubles on every attempt up to MaxInitialRetryDelay.
	// Zero (or negative) selects DefaultInitialRetryDelay.
	InitialRetryDelay time.Duration

	// InitialRetryAttempts is how many times a failed initial collection is
	// retried before the daemon falls back to the regular interval.
	// Zero selects DefaultInitialRetryAttempts; a negative value disables
	// initial-run retries entirely.
	InitialRetryAttempts int
}

// DefaultDaemonConfig returns default daemon configuration.
func DefaultDaemonConfig() DaemonConfig {
	return DaemonConfig{
		Interval: 24 * time.Hour,
	}
}

// Daemon runs TechPulse in continuous collection mode.
type Daemon struct {
	baseConfig Config // Initial config (used if no ConfigProvider)
	config     DaemonConfig
	log        logger.Logger
}

// NewDaemon creates a new daemon instance.
func NewDaemon(baseCfg Config, daemonCfg DaemonConfig) *Daemon {
	return &Daemon{
		baseConfig: baseCfg,
		config:     daemonCfg,
		log:        logger.Default(),
	}
}

// getConfig returns the current config (from provider or base config).
func (d *Daemon) getConfig() Config {
	if d.config.ConfigProvider != nil {
		return d.config.ConfigProvider()
	}
	return d.baseConfig
}

// SetLogger sets a custom logger.
func (d *Daemon) SetLogger(l logger.Logger) {
	d.log = l
}

// validateInterval reports whether d is safe for time.NewTicker.
func validateInterval(d time.Duration) error {
	if d <= 0 {
		return fmt.Errorf("interval must be positive, got %s", d)
	}
	return nil
}

// newTicker creates a ticker after checking the interval, so an invalid value
// surfaces as a config error instead of a panic inside time.NewTicker.
func newTicker(interval time.Duration) (*time.Ticker, error) {
	if err := validateInterval(interval); err != nil {
		return nil, err
	}
	return time.NewTicker(interval), nil
}

// initialRetryDelay returns the configured base retry delay.
func (d *Daemon) initialRetryDelay() time.Duration {
	if d.config.InitialRetryDelay <= 0 {
		return DefaultInitialRetryDelay
	}
	return d.config.InitialRetryDelay
}

// initialRetryAttempts returns how many times to retry the initial collection.
func (d *Daemon) initialRetryAttempts() int {
	switch {
	case d.config.InitialRetryAttempts < 0:
		return 0
	case d.config.InitialRetryAttempts == 0:
		return DefaultInitialRetryAttempts
	default:
		return d.config.InitialRetryAttempts
	}
}

// sleepCtx waits for d, returning early (false) if ctx is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// collectWithRetry runs the initial collection, retrying with bounded
// exponential backoff so a startup failure does not idle for a full interval.
// It reports whether some attempt succeeded.
func (d *Daemon) collectWithRetry(ctx context.Context) bool {
	if err := d.collect(ctx); err == nil {
		return true
	}

	attempts := d.initialRetryAttempts()
	delay := d.initialRetryDelay()
	for attempt := 1; attempt <= attempts; attempt++ {
		d.log.Warn("Collection failed; retrying",
			logger.F("attempt", attempt),
			logger.F("max_attempts", attempts),
			logger.F("retry_in", delay.String()),
		)
		if !sleepCtx(ctx, delay) {
			return false
		}
		delay *= 2
		if delay > MaxInitialRetryDelay {
			delay = MaxInitialRetryDelay
		}
		if err := d.collect(ctx); err == nil {
			return true
		}
	}

	d.log.Error("Initial collection failed after retries; next attempt at regular interval",
		logger.F("attempts", attempts))
	return false
}

// Run starts the daemon and blocks until context is cancelled.
// Returns the number of successful collections and any error.
func (d *Daemon) Run(ctx context.Context) (int, error) {
	if err := validateInterval(d.config.Interval); err != nil {
		err = fmt.Errorf("invalid daemon config: %w", err)
		d.log.Error("Daemon cannot start", logger.F("error", err))
		return 0, err
	}

	d.log.Info("Daemon starting", logger.F("interval", d.config.Interval.String()))
	if d.config.OnStart != nil {
		d.config.OnStart()
	}

	collections := 0

	// Run immediately on start, with short retries on failure.
	if d.collectWithRetry(ctx) {
		collections++
	}

	ticker, err := newTicker(d.config.Interval)
	if err != nil {
		// Unreachable because of the validation above; kept defensive so
		// every NewTicker call is guarded.
		return collections, fmt.Errorf("invalid daemon config: %w", err)
	}
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.log.Info("Daemon stopped", logger.F("collections", collections))
			if d.config.OnStop != nil {
				d.config.OnStop()
			}
			return collections, ctx.Err()
		case <-ticker.C:
			if err := d.collect(ctx); err == nil {
				collections++
			}
		}
	}
}

// collect performs a single collection cycle with current config.
func (d *Daemon) collect(ctx context.Context) error {
	if d.config.OnTick != nil {
		d.config.OnTick()
	}

	cfg := d.getConfig()
	tp := d.createTechPulse(cfg)
	tp.SetLogger(d.log)

	d.log.Info("Starting collection", logger.F("time", time.Now().Format(time.RFC3339)))
	err := tp.Run(ctx)

	if d.config.OnDone != nil {
		d.config.OnDone()
	}

	if err != nil {
		d.log.Error("Collection failed", logger.F("error", err))
		return err
	}

	d.log.Info("Collection completed")
	return nil
}

// createTechPulse creates a TechPulse instance using factory or default.
func (d *Daemon) createTechPulse(cfg Config) *TechPulse {
	if d.config.TechPulseFactory != nil {
		return d.config.TechPulseFactory(cfg)
	}
	return NewWithOptions(cfg)
}
