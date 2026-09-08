# TechPulse

A CLI tool that automatically collects and curates AI/tech news from multiple sources.

## Features

- **7 Data Sources**: Hacker News (Top/Ask/Show), RSS feeds, GitHub Trending, Reddit, Lobsters
- **Smart Filtering**: Keyword matching with include/exclude rules
- **Deduplication**: Removes duplicate articles across sources
- **Importance Scoring**: Ranks articles by score, comments, and keywords
- **Source Weighting**: Prioritizes high-quality sources
- **Content Summaries**: Extract article content summaries (`--summary`)
- **Published Dates**: Shows article publication time in reports
- **Rate Limiting**: Per-domain throttling to prevent API bans
- **Daemon Mode**: Continuous collection at configurable intervals, with hot-reload support
- **Progress Bar**: Visual feedback during collection (`--progress`)
- **Markdown Reports**: Clean, readable daily digests

## Quick Start

```bash
# Build the tool
go build -o techpulse ./cmd/techpulse

# Run once (collects from all sources)
./techpulse

# View the report
cat .techpulse/DIGEST.md
```

## Installation

Requires Go 1.22+.

```bash
git clone https://github.com/majiayu000/techpulse.git
cd techpulse
go build -o techpulse ./cmd/techpulse
```

## Usage

### Basic Commands

```bash
# Collect from all sources
./techpulse

# List available sources
./techpulse --list-sources

# Collect from specific sources
./techpulse --sources hackernews_top,github_trending_daily

# Limit articles per source
./techpulse --limit 20

# Custom output directory
./techpulse --output ./my-reports
```

### Daemon Mode

Run continuously with automatic collection:

```bash
# Collect every hour
./techpulse --daemon --interval 1h

# Collect every 6 hours
./techpulse --daemon --interval 6h

# Default: collect every 24 hours
./techpulse --daemon
```

Press `Ctrl+C` to stop gracefully.

**Hot Reload**: In daemon mode, configuration changes are automatically applied on the next collection cycle. Simply edit your `techpulse.yaml` file while the daemon is running.

### Configuration File

Generate an example config:

```bash
./techpulse --gen-config
```

This creates `techpulse.yaml`:

```yaml
# Maximum articles per source
limit: 30

# Output directory
output: .techpulse

# Request timeout in seconds
timeout: 60

# Archive retention in days (default: 30). Set to 0 to disable cleanup.
# retention_days: 30

# Sources to use (optional, uses all if empty)
# sources:
#   - hackernews_top
#   - github_trending_daily

# Keyword filters
keywords:
  include:
    - AI
    - LLM
    - GPT
    - Claude
    - machine learning
    - Rust
    - Go
  exclude:
    - crypto
    - NFT
    - blockchain

# Custom RSS feeds
# rss_feeds:
#   - name: My Feed
#     url: https://example.com/feed.xml

# Enable content summary extraction (default: false)
enable_summary: false
```

Config file locations (in order):
1. `techpulse.yaml` (current directory)
2. `.techpulse/config.yaml`
3. `~/.config/techpulse/config.yaml`

CLI flags override config file settings.

## Data Sources

| Source | ID | Description |
|--------|------|-------------|
| Hacker News Top | `hackernews_top` | Top stories from HN |
| Hacker News Ask | `hackernews_ask` | Ask HN discussions |
| Hacker News Show | `hackernews_show` | Show HN projects |
| RSS | `rss` | TechCrunch, Ars Technica, The Verge, Wired, MIT Technology Review |
| GitHub Trending | `github_trending_daily` | Daily trending repositories |
| Reddit | `reddit` | r/MachineLearning, r/programming, etc. |
| Lobsters | `lobsters_hottest` | Hottest from lobste.rs |

## Output

Reports are saved to `.techpulse/` by default:

```
.techpulse/
  DIGEST.md           # Latest report
  archive/
    2026-01-01.md     # Archived daily reports
```

### Report Format

```markdown
# Daily Tech Digest - 2026-01-01

Collected **64** articles | Avg Importance: **6.5**/10

## Top Stories

### 1. [Article Title](https://example.com)
- Source: hackernews_top | Score: 234 | Importance: 8.5/10 | Published: 2026-01-01 10:30
- Keywords: [AI LLM]

> Brief summary of the article content...

```

## CLI Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--limit` | 30 | Max articles per source |
| `--sources` | all | Comma-separated source IDs |
| `--output` | .techpulse | Output directory |
| `--timeout` | 60 | Request timeout (seconds) |
| `--config` | auto | Path to config file |
| `--gen-config` | - | Generate example config |
| `--list-sources` | - | List available sources |
| `--daemon` | false | Run continuously |
| `--interval` | 24h | Collection interval (daemon) |
| `--quiet`, `-q` | false | Suppress all output except errors |
| `--progress` | false | Show collection progress bar |
| `--summary` | false | Extract content summaries from articles |
| `--validate` | - | Validate config file and exit |
| `--version`, `-v` | - | Show version information |

## Development

### Project Structure

```
cmd/techpulse/          # CLI entry point
internal/
  collector/            # Data source collectors
    hackernews/         # Hacker News API
    rss/                # RSS feed parser
    github/             # GitHub Trending scraper
    reddit/             # Reddit API
    lobsters/           # Lobsters API
  filter/               # Content filtering
  summarizer/           # Scoring and ranking
  extractor/            # Content summary extraction
  storage/              # Markdown output
  httpclient/           # HTTP with retries & rate limiting
  logger/               # Structured logging
  progress/             # Terminal progress bar
  techpulse/            # Main orchestrator
```

### Running Tests

```bash
# Run all tests
go test ./...

# Using Makefile
make test              # Run all tests
make test-cover        # Run with coverage summary
make test-cover-report # Generate coverage.out file
make test-cover-html   # Generate HTML report and open in browser
```

### Test Coverage

Measured with `go test -cover` (statement-weighted total via `go tool cover`):

| Module | Coverage |
|--------|----------|
| filter | 97.6% |
| summarizer | 95.9% |
| reddit | 94.4% |
| extractor | 92.6% |
| rss | 92.1% |
| progress | 91.9% |
| techpulse | 91.7% |
| hackernews | 90.3% |
| httpclient | 89.5% |
| github | 87.4% |
| storage | 87.1% |
| lobsters | 87.0% |
| logger | 85.7% |
| memory | 75.9% |
| collector (registry) | 65.5% |
| cmd/techpulse | 63.7% |
| **Total** | **88.2%** |

### Make Commands

```bash
make build            # Build the techpulse binary (dev)
make build-release    # Build with version info embedded
make test             # Run all tests
make test-cover       # Run tests with coverage summary
make test-cover-html  # Generate HTML coverage report
make fmt              # Format code
make vet              # Run go vet
make run              # Build and run
make run-daemon       # Build and run in daemon mode
make clean            # Remove build artifacts
make help             # Show all commands
```

### Code Standards

- Each `.go` file is under 200 lines
- Clean module boundaries with interfaces
- Comprehensive test coverage (336 tests, 88% statement coverage)

## License

MIT
