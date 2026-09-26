package techpulse

import (
	"context"
	"fmt"
	"time"

	"github.com/majiayu000/techpulse/internal/collector"
	"github.com/majiayu000/techpulse/internal/logger"
	"github.com/majiayu000/techpulse/internal/progress"
)

// collectFromSources collects from specific sources or all if none specified.
func (tp *TechPulse) collectFromSources(ctx context.Context, opts collector.Options) []collector.Result {
	if len(tp.config.Sources) > 0 {
		return tp.collectFromSpecified(ctx, opts)
	}
	return tp.collectFromAll(ctx, opts)
}

func (tp *TechPulse) collectFromAll(ctx context.Context, opts collector.Options) []collector.Result {
	total := len(tp.registry.All())
	tp.log.Info("Collecting from sources...", logger.F("count", total))

	bar := progress.New(total)
	bar.SetDisabled(!tp.showProgress)
	bar.Start()

	cb := collector.ProgressCallback{
		OnStart:    func(name string) { bar.StartTask(name) },
		OnComplete: func(name string, n int) { bar.CompleteTask(name, fmt.Sprintf("%d articles", n)) },
		OnError:    func(name string, err error) { bar.FailTask(name, err.Error()) },
	}

	results := tp.registry.CollectAllWithProgress(ctx, opts, cb)
	bar.Finish()
	return results
}

func (tp *TechPulse) collectFromSpecified(ctx context.Context, opts collector.Options) []collector.Result {
	tp.log.Info("Collecting from specified sources...", logger.F("count", len(tp.config.Sources)))
	bar := progress.New(len(tp.config.Sources))
	bar.SetDisabled(!tp.showProgress)
	bar.Start()

	var results []collector.Result
	for _, name := range tp.config.Sources {
		c, ok := tp.registry.Get(name)
		if !ok {
			tp.log.Warn("Unknown source, skipping", logger.F("source", name))
			continue
		}
		bar.StartTask(name)
		start := time.Now()
		articles, err := c.Collect(ctx, opts)
		r := collector.Result{
			Source:    name,
			Articles:  articles,
			Error:     err,
			Duration:  time.Since(start),
			Timestamp: time.Now(),
		}
		results = append(results, r)
		if err != nil {
			bar.FailTask(name, err.Error())
		} else {
			bar.CompleteTask(name, fmt.Sprintf("%d articles", len(articles)))
		}
	}
	bar.Finish()
	return results
}

func (tp *TechPulse) combineResults(results []collector.Result) []collector.Article {
	var all []collector.Article
	for _, r := range results {
		if r.Error != nil {
			tp.log.Error("Collection failed",
				logger.F("source", r.Source), logger.F("error", r.Error))
			continue
		}
		tp.log.Info("Collected", logger.F("source", r.Source),
			logger.F("count", len(r.Articles)), logger.F("duration", r.Duration.Seconds()))
		all = append(all, r.Articles...)
	}
	return all
}
