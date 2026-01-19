# Project Structure

```
conqr/
├── src/                    # TypeScript source
│   ├── index.ts           # Entry point - orchestrates startup
│   ├── cli.ts             # CLI argument parsing
│   ├── config.ts          # Config file loading (conqr.json)
│   ├── process-manager.ts # Process spawning, lifecycle, events
│   ├── log-buffer.ts      # Log storage (per-process + unified)
│   └── ui.tsx             # Ink/React TUI components
├── dist/                   # Compiled output (gitignored content)
├── demo/                   # Demo logger scripts for testing
├── conqr.schema.json      # JSON schema for config validation
└── conqr.json.example*    # Example config files
```

## Architecture

### Data Flow
1. `index.ts` - Parses CLI or loads config, creates managers, renders UI
2. `cli.ts` / `config.ts` - Parse commands into `CommandInfo[]`
3. `process-manager.ts` - Spawns processes, emits `log` and `status-change` events
4. `log-buffer.ts` - Stores logs per-process and unified (max 1000 lines/process)
5. `ui.tsx` - React components subscribe to events, render TUI

### Key Types
- `CommandInfo`: `{ id, name, command }`
- `ProcessStatus`: `'running' | 'stopped' | 'error' | 'unknown'`
- `LogEntry`: `{ line, source, timestamp, processId? }`

### UI Components
- `TUI` - Main container, keyboard/mouse handling
- `Sidebar` - Process list with status indicators
- `MainPane` - Log viewer with scrolling
- `LogsList` - Renders log entries with ANSI color detection

### Event System
`ProcessManager` extends `EventEmitter`:
- `log`: New log line from process
- `status-change`: Process status changed
