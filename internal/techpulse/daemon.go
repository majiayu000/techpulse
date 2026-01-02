// Package techpulse daemon mode support.
package techpulse

import (
	"context"
	"time"

	"github.com/anthropic/autonomous-runner/internal/logger"
)

// DaemonConfig contains configuration for daemon mode.
type DaemonConfig struct {
	Interval         time.Duration               // Collection interval
	ConfigProvider   func() Config               // Returns current config (for hot-reload)
	TechPulseFactory func(Config) *TechPulse     // Custom factory (for testing)
	OnStart          func()                      // Called when daemon starts
	OnTick           func()                      // Called before each collection
	OnDone           func()                      // Called after each collection
	OnStop           func()                      // Called when daemon stops
	OnConfigReload   func(old, new Config)       // Called when config is reloaded
}

// DefaultDaemonConfig returns default daemon configuration.
func DefaultDaemonConfig() DaemonConfig {
	return DaemonConfig{
		Interval: 24 * time.Hour,
	}
}

// Daemon runs TechPulse in continuous collection mode.
type Daemon struct {
	baseConfig Config       // Initial config (used if no ConfigProvider)
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

// Run starts the daemon and blocks until context is cancelled.
// Returns the number of successful collections and any error.
func (d *Daemon) Run(ctx context.Context) (int, error) {
	d.log.Info("Daemon starting", logger.F("interval", d.config.Interval.String()))
	if d.config.OnStart != nil {
		d.config.OnStart()
	}

	collections := 0

	// Run immediately on start
	if err := d.collect(ctx); err == nil {
		collections++
	}

	ticker := time.NewTicker(d.config.Interval)
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
