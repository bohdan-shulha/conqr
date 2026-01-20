# conqr

Dead-simple TUI process runner for Node.js.

## Usage

### Command Line Arguments

```bash
conqr 'npm run dev' 'npm run build:emails' 'npm run worker'
```

You can customize process names using the `'name'='command'` syntax:

```bash
conqr 'Dev Server'='npm run dev' 'Build Process'='npm run build' 'Worker'='npm run worker'
```

### Configuration File

Create a `conqr.json` or `.conqr.json` file in your project directory:

**Simple commands (string values):**
```json
{
  "commands": {
    "Dev Server": "npm run dev",
    "Build Process": "npm run build",
    "Worker": "npm run worker"
  }
}
```

**Extended commands (with restart options):**
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

**Mixed simple and extended:**
```json
{
  "commands": {
    "Dev Server": "npm run dev",
    "Worker": {
      "command": "npm run worker",
      "restart": { "policy": "on-error" }
    }
  }
}
```

Then simply run:
```bash
conqr
```

CLI arguments take precedence over the config file if both are provided.

### Restart Configuration

Configure automatic restart behavior for processes that crash or exit.

**Restart policies:**
- `"never"` - No automatic restart (default)
- `"on-error"` - Restart only when process exits with non-zero code
- `"on-exit"` - Restart whenever process exits, regardless of exit code

**Global restart (applies to all commands):**
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

**Per-process restart (overrides global):**
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

For IDE autocomplete and validation, add a `$schema` reference to your config file:

```json
{
  "$schema": "https://raw.githubusercontent.com/bohdan-shulha/conqr/main/conqr.schema.json",
  "commands": {
    "Dev Server": "npm run dev",
    "Build": "npm run build"
  }
}
```

The schema file is also available in the npm package at `node_modules/conqr/conqr.schema.json` for local reference.

## Demo

Try it with the included demo scripts:

```bash
npm start 'node demo/logger1.js' 'node demo/logger2.js' 'node demo/logger3.js'
```

## Features

- Run multiple commands concurrently
- Two-pane interface:
  - **Sidebar**: "All processes" menu item and list of commands with status indicators (UP = running, ERROR = error detected, DOWN = stopped)
  - **Main pane**: Logs from selected command or unified view when "All processes" is selected
- ANSI color support in logs
- Automatic error detection based on log patterns and ANSI color codes
- Auto-scroll to bottom when new logs arrive (can be disabled by scrolling up)
- Mouse wheel scrolling support
- Raw mode for full-screen log viewing (press `r` to toggle)
- Automatic process restart with configurable policies (`never`, `on-error`, `on-exit`)
- Keyboard controls:
  - **Arrow Left/Right**: Switch focus between sidebar and main pane
  - **Arrow Up/Down**:
    - In sidebar: Navigate between commands (including "All processes" menu item)
    - In main pane: Scroll logs line by line
  - **PageUp/PageDown**: Scroll logs 10 lines at a time (main pane)
  - **Home**: Jump to top of logs (main pane)
  - **End**: Jump to bottom of logs (main pane)
  - **r**: Restart selected process (sidebar)
  - **l**: Toggle raw mode (full-screen log view)
  - **q** or **Ctrl+C**: Quit application

## Requirements

- Node.js >= 18.0.0

## Installation

Install globally:
```bash
npm install -g conqr
```

Or install locally in your project:
```bash
npm install conqr
```

## Build

```bash
npm run build
```

## Development

Run in development mode (using tsx):
```bash
npm start 'command1' 'command2' 'command3'
```

Or after building:
```bash
npm run build
node dist/index.js 'command1' 'command2' 'command3'
```

Or after installing globally:
```bash
conqr 'command1' 'command2' 'command3'
```
