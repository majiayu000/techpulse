# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | Yes       |

## Reporting a Vulnerability

1. **Do not** open a public issue
2. Use [GitHub Security Advisories](https://github.com/majiayu000/techpulse/security/advisories/new)
3. Include steps to reproduce and potential impact

## Claude CLI permission mode

By default, `internal/worker.Runner` invokes the Claude CLI **without**
`--dangerously-skip-permissions`. Permission prompts / CLI policy remain in
effect so the agent does not get unrestricted local tool use by default.

### Opt-in (explicit, high risk)

Skip-permissions can be enabled only via an explicit opt-in:

- Go: set `Runner.SkipPermissions = true` (or `SetSkipPermissions(true)`)
- Env: `TECHPULSE_CLAUDE_SKIP_PERMISSIONS=true` (preferred)
- Env alias: `AUTONOMOUS_RUNNER_SKIP_PERMISSIONS=true`

Accepted truthy values: `1`, `true`, `yes`, `on` (case-insensitive).

When enabled, the worker logs a warning and appends
`--dangerously-skip-permissions`. That flag disables filesystem/tool permission
prompts and can allow broad local execution in (and potentially beyond) the
workspace, depending on Claude CLI policy.

### Residual risk

Even with the default (permission-prompt) path:

- The agent can still modify files and run commands that the CLI allows after
  approval or under its normal sandbox rules.
- Opt-in skip-permissions removes an important guardrail; use it only in
  isolated, disposable workspaces you fully trust.
- This project does not yet enforce a hard tool allowlist or OS-level sandbox
  beyond Claude CLI permission mode. Treat unattended runs as privileged.
