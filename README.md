# Auto Run Agent (Go)

A Go implementation of the Autonomous Runner for Claude Code. It orchestrates a loop that reads tasks and context from `memory/`, runs the Claude CLI to make progress in `workspace/`, and detects progress via memory hashes and optional Git status.

## Features
- Iteration / cost / duration limits
- Memory files: `TASKS.md`, `CONTEXT.md`, `DONE.md`
- Progress detection: memory hash + Git
- Logs to `logs/`

## Project Layout
```
project/
├── memory/        # TASKS.md, CONTEXT.md, DONE.md
├── workspace/     # working directory for Claude
└── logs/          # orchestrator logs
```

## Usage
Build and run:
```
go build -o orchestrator ./cmd/orchestrator/
./orchestrator --dir /path/to/project
```

Or use the helper script:
```
./run.sh [max_iterations] [max_cost] [max_duration_hours] [project_dir]
```

## Requirements
- Go 1.22+
- Claude Code CLI (`claude`)

## Config
The orchestrator loads `config.yaml` from the project directory if present; otherwise defaults are used.

## Notes
This repo expects the runtime project directory to contain `memory/`, `workspace/`, and `logs/`. The orchestrator will create missing folders.
