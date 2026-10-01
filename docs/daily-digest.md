# Build a Hacker News and RSS daily digest

TechPulse produces a local Markdown report from collected articles. This example
uses Hacker News Top and RSS, with an extra feed from the official Go blog.
It requires the [source installation](../README.md#installation) and access to
the selected public feeds.

## Choose sources and keywords

Save this as `daily.yaml` in your checkout.

```yaml
limit: 10
output: ./daily-reports
timeout: 20
sources:
  - hackernews_top
  - rss
keywords:
  include: [AI, OpenAI, LLM, Go, Golang, Rust]
  exclude: [sponsored, crypto]
rss_feeds:
  - name: Go Blog
    url: https://go.dev/blog/feed.atom
enable_summary: false
```

Custom RSS feeds are appended to the built-in RSS sources. They do not replace
that list. `limit` applies to each collector; it is not a guarantee of the final
article count after filtering and deduplication.

## Validate and generate the report

```bash
./techpulse --config daily.yaml --validate
./techpulse --config daily.yaml --progress
cat daily-reports/DIGEST.md
```

Validation checks configuration without collecting articles. A completed run
writes `DIGEST.md` and a dated file under `daily-reports/archive/`. The report
contains source links and matched keywords. Counts vary with the feeds and
network state; the example is not a fixed benchmark.

To repeat collection hourly while this process stays running, use
`./techpulse --config daily.yaml --daemon --interval 1h`. Stop it with `Ctrl+C`.
The daemon applies configuration changes on its next collection cycle.

## Troubleshoot an empty or unexpected digest

- Check the source IDs with `./techpulse --list-sources` and read collection
  errors in the terminal. A valid configuration does not prove a feed is online.
- Include keywords must match at least one term; an exclude match removes the
  article. Single-word English terms match whole tokens. For example, `Go` does
  not match `Golang`, and `AI` does not match `OpenAI`; list both when needed.
- To inspect a broader feed sample, set `include: []` and `exclude: []` in the
  explicit config and run it again. Restore your filters after checking results.
- `--summary` extracts and truncates source text. It does not call a language
  model. A missing summary can reflect unavailable or unextractable content.
- Use `./techpulse` for news collection. The repository's `./run.sh` starts
  the separate [autonomous Claude CLI runner](autonomous-runner.md).

The [CLI reference](../README.md#cli-reference) lists other options. Source
installation is documented in this repository; `make build-release` embeds
Git-derived version information for bug reports. See the [MIT license](../LICENSE)
and [support instructions](../README.md#support).
