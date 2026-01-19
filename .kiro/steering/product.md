# Product Overview

**conqr** is a dead-simple TUI (Terminal User Interface) process runner for Node.js.

## Purpose
Run and monitor multiple concurrent processes in a single terminal with a two-pane interface.

## Key Features
- Run multiple commands concurrently from CLI args or config file
- Two-pane interface: sidebar (process list with status) + main pane (logs)
- "All processes" unified log view
- ANSI color support in logs
- Automatic error detection (log patterns + ANSI codes)
- Auto-scroll with manual scroll override
- Mouse wheel scrolling
- Raw mode for full-screen log viewing
- Process restart capability

## Status Indicators
- **UP** (green): Process running normally
- **ERROR** (orange): Error detected in logs or process failed
- **DOWN** (red): Process stopped

## Input Methods
1. CLI arguments: `conqr 'npm run dev' 'npm run build'`
2. Named commands: `conqr 'Dev'='npm run dev'`
3. Config file: `conqr.json` or `.conqr.json`
