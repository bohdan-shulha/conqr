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
- Two-pane terminal interface with process statuses and logs
- Unified "All processes" log view
- ANSI color support in logs
- Automatic error detection from common error patterns and red ANSI output
- Auto-scroll to bottom while new logs arrive
- Mouse wheel and keyboard scrolling
- Raw log mode with `l`
- Manual process restart with `r`
- Automatic process restart with configurable policies
- Graceful process-group shutdown on quit

## Keyboard Controls

- Arrow Left/Right: switch focus between sidebar and logs
- Arrow Up/Down: navigate commands or scroll logs
- PageUp/PageDown: scroll logs 10 lines
- Home/End: jump to top or bottom
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
