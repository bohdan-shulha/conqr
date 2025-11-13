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

**Array of commands:**
```json
{
  "commands": [
    "npm run dev",
    "npm run build",
    "npm run worker"
  ]
}
```

**Object with custom names:**
```json
{
  "commands": {
    "Dev Server": "npm run dev",
    "Build Process": "npm run build",
    "Worker": "npm run worker"
  }
}
```

**Array of objects:**
```json
{
  "commands": [
    {
      "name": "Dev Server",
      "command": "npm run dev"
    },
    {
      "name": "Build Process",
      "command": "npm run build"
    }
  ]
}
```

Then simply run:
```bash
conqr
```

CLI arguments take precedence over the config file if both are provided.

### JSON Schema

For IDE autocomplete and validation, add a `$schema` reference to your config file:

```json
{
  "$schema": "https://raw.githubusercontent.com/bohdan-shulha/conqr/main/conqr.schema.json",
  "commands": [
    "npm run dev",
    "npm run build"
  ]
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
  - **Sidebar**: "All processes" menu item and list of commands with status indicators (▲ = running, ▼ = stopped/error)
  - **Main pane**: Logs from selected command or unified view when "All processes" is selected
- Keyboard controls:
  - **Arrow Up/Down**: Navigate between commands (including "All processes" menu item)
  - **q**: Quit application

## Installation

```bash
npm install
```

## Build

```bash
npm run build
```

## Run

Development (using tsx):
```bash
npm start 'command1' 'command2' 'command3'
```

Or after building:
```bash
node dist/index.js 'command1' 'command2' 'command3'
```

Or after installing globally:
```bash
conqr 'command1' 'command2' 'command3'
```
