// Configuration file watcher for hot-reload support.
package techpulse

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/anthropic/autonomous-runner/internal/logger"
)

// ConfigWatcher watches a configuration file for changes.
type ConfigWatcher struct {
	path     string
	interval time.Duration
	log      logger.Logger
	lastMod  time.Time
	mu       sync.RWMutex
	onChange func(*FileConfig)
}

// WatcherConfig contains watcher configuration options.
type WatcherConfig struct {
	Path     string        // Config file path to watch
	Interval time.Duration // Check interval (default: 5s)
	OnChange func(*FileConfig)
}

// DefaultWatcherInterval is the default file check interval.
const DefaultWatcherInterval = 5 * time.Second

// NewConfigWatcher creates a new configuration file watcher.
func NewConfigWatcher(cfg WatcherConfig) *ConfigWatcher {
	if cfg.Interval == 0 {
		cfg.Interval = DefaultWatcherInterval
	}
	return &ConfigWatcher{
		path:     cfg.Path,
		interval: cfg.Interval,
		log:      logger.Default(),
		onChange: cfg.OnChange,
	}
}

// SetLogger sets a custom logger.
func (w *ConfigWatcher) SetLogger(l logger.Logger) {
	w.log = l
}

// Watch starts watching the config file until context is cancelled.
func (w *ConfigWatcher) Watch(ctx context.Context) error {
	// Get initial modification time
	info, err := os.Stat(w.path)
	if err != nil {
		return err
	}
	w.mu.Lock()
	w.lastMod = info.ModTime()
	w.mu.Unlock()

	w.log.Info("Watching config file", logger.F("path", w.path))

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if w.checkAndReload() {
				w.log.Info("Config reloaded", logger.F("path", w.path))
			}
		}
	}
}

// checkAndReload checks if file changed and reloads if needed.
func (w *ConfigWatcher) checkAndReload() bool {
	info, err := os.Stat(w.path)
	if err != nil {
		w.log.Warn("Cannot stat config", logger.F("error", err))
		return false
	}

	w.mu.RLock()
	changed := info.ModTime().After(w.lastMod)
	w.mu.RUnlock()

	if !changed {
		return false
	}

	cfg, err := LoadConfigFile(w.path)
	if err != nil {
		w.log.Error("Failed to reload config", logger.F("error", err))
		return false
	}

	w.mu.Lock()
	w.lastMod = info.ModTime()
	w.mu.Unlock()

	if w.onChange != nil {
		w.onChange(cfg)
	}
	return true
}

// LastModified returns the last known modification time.
func (w *ConfigWatcher) LastModified() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastMod
}
