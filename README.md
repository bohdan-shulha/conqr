# conqr

Dead-simple TUI process runner, written in Go.

## Usage

### Command Line Arguments

```bash
conqr 'npm run dev' 'npm run build:emails' 'npm run worker'
```

Customize process names with the `'name'='command'` syntax:

```bash
conqr 'Dev Server'='npm run dev' 'Build Process'='npm run build' 'Worker'='npm run worker'
```

### Configuration File

Create a `conqr.json` or `.conqr.json` file in your project directory.

Simple commands:

```json
{
  "commands": {
    "Dev Server": "npm run dev",
    "Build Process": "npm run build",
    "Worker": "npm run worker"
  }
}
```

Extended commands with restart options:

```json
{
  "commands": {
    "Dev Server": {
      "command": "npm run dev",
      "restart": {
        "policy": "on-error",
        "delay": 2000
      }
    },
    "Worker": {
      "command": "npm run worker",
      "restart": {
        "policy": "on-exit",
        "delay": 5000
      }
    }
  }
}
```

Then run:

```bash
conqr
```

CLI arguments take precedence over config files.

### Restart Configuration

Restart policies:

- `"never"`: no automatic restart
- `"on-error"`: restart only when a process exits with a non-zero code
- `"on-exit"`: restart whenever a process exits

Global restart settings apply to all config-file commands:

```json
{
  "restart": {
    "policy": "on-error",
    "delay": 2000
  },
  "commands": {
    "Dev Server": "npm run dev",
    "Worker": "npm run worker"
  }
}
```

Per-process restart settings override global settings:

```json
{
  "restart": {
    "policy": "on-error",
    "delay": 2000
  },
  "commands": {
    "Dev Server": "npm run dev",
    "Worker": {
      "command": "npm run worker",
      "restart": {
        "policy": "on-exit",
        "delay": 5000
      }
    }
  }
}
```

### Process Groups

Group commands in the sidebar and view merged logs per group. Use the object form for grouped commands:

```json
{
  "defaultGroup": "services",
  "commands": {
    "api": { "command": "npm run api", "group": "services" },
    "ssr": { "command": "npm run ssr", "group": "services" },
    "worker": { "command": "npm run worker", "group": "services" },
    "cf": { "command": "cloudflared tunnel run", "group": "tunnels" },
    "emails": { "command": "npm run build:emails", "group": "build" }
  }
}
```

- `group`: optional label used for sidebar sections and merged log views
- `defaultGroup`: optional group selected on startup (falls back to "All processes" if missing)
- Simple string commands cannot carry a `group`; use the object form instead
- Select a group row to see merged logs from all processes in that group
- Select an indented process row to see logs for one process
- Stop/restart (`s`/`r`) work on individual process rows only

### Startup Dependencies

Watch-mode services start at the same time by default. In a monorepo this makes a rebuild
waterfall: each package compiles against an incomplete build of the package below it, and then
compiles again when that package writes its output.

Use `dependsOn` and `ready` to start each service only after its dependencies finish the first
build:

```json
{
  "ready": "Found 0 errors\\. Watching for file changes",
  "busy": "File change detected",
  "commands": {
    "core": { "command": "tsc -w -p packages/core" },
    "db": { "command": "tsc -w -p packages/db", "dependsOn": ["core"] },
    "emails": { "command": "npm run build:emails", "ready": "" },
    "api": { "command": "tsc -w -p apps/api", "dependsOn": ["core", "db"] },
    "web": {
      "command": "vite dev",
      "dependsOn": ["core", "emails"],
      "ready": "ready in \\d+ ms"
    }
  }
}
```

- `dependsOn`: names of other commands that must become ready first
- `ready`: regular expression that marks the command as ready
- `busy`: regular expression that marks the command as busy again
- `readyTimeout`: milliseconds to wait for a dependency (default 120000)

Every command that has no unmet dependency starts at once. Each other command starts as soon as its
dependencies report ready.

A command becomes ready in one of three ways:

1. An output line matches the `ready` pattern.
2. The command exits with code 0. Use this for a one-shot build step.
3. The `readyTimeout` expires. conqr then writes a warning and starts the dependents.

A non-zero exit does not make a command ready. The dependents wait for the timeout.

Rules for the patterns:

- Global `ready` and `busy` apply to all commands. A per-command value overrides the global value.
- An empty string clears an inherited global pattern.
- conqr removes the ANSI colour codes from a line before it matches the pattern.
- conqr tests `busy` before `ready`.
- A long-running dependency needs a `ready` pattern. Without one it gates the dependents until the
  timeout.

Sidebar labels:

| Label | Meaning |
|---|---|
| `WAIT` | The command waits for a dependency |
| `BUILD` | The command runs and builds now |
| `UP` | The command runs and is ready |
| `ERROR` | conqr found an error pattern in the recent output |
| `DOWN` | The command is not running |
| `STOP` | You stopped the command with `s` |

conqr removes the startup waterfall. conqr cannot remove the rebuild cascade after startup, because
each watch process rebuilds on its own. To remove that cascade, replace the separate watch processes
with one incremental build, for example `tsc -b --watch` over TypeScript project references.

### JSON Schema

For IDE autocomplete and validation, add a `$schema` reference:

```json
{
  "$schema": "https://raw.githubusercontent.com/bohdan-shulha/conqr/main/conqr.schema.json",
  "commands": {
    "Dev Server": "npm run dev",
    "Build": "npm run build"
  }
}
```

## Demo

Try it with the included demo scripts:

```bash
go run . 'node demo/logger1.js' 'node demo/logger2.js' 'node demo/logger3.js'
```

## Features

- Run multiple commands concurrently
- Dependency-ordered startup with readiness detection
- Live build state for watch-mode processes
- Two-pane terminal interface with process statuses and logs
- Process groups with merged log views per group
- Unified "All processes" log view
- ANSI color support in logs
- Automatic error detection from common error patterns and red ANSI output
- Auto-scroll to bottom while new logs arrive
- Mouse wheel and keyboard scrolling
- Raw log mode with `l`
- Manual process stop with `s`
- Manual process restart with `r`
- Automatic process restart with configurable policies
- Graceful process-group shutdown on quit

## Keyboard Controls

- Arrow Left/Right: switch focus between sidebar and logs
- Arrow Up/Down: navigate commands or scroll logs
- PageUp/PageDown: scroll logs 10 lines
- Home/End: jump to top or bottom
- `s`: stop selected process
- `r`: restart selected process
- `l`: toggle raw log mode
- `q` or Ctrl+C: quit

## Requirements

- For npm installs: Node.js 18 or newer
- For source builds: Go 1.25 or newer
- Prebuilt npm binaries are published for macOS, Linux, and Windows on `amd64` and `arm64`

## Installation

Install from npm:

```bash
npm install -g conqr
```

The npm package preserves the existing `conqr` command and launches the bundled Go binary for your platform.

Install from source:

```bash
go install github.com/bohdan-shulha/conqr@latest
```

Or build locally:

```bash
make build
./bin/conqr 'command1' 'command2' 'command3'
```

## Development

```bash
make test
go run . 'command1' 'command2' 'command3'
```
