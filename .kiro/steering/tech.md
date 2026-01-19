# Tech Stack

## Runtime
- Node.js >= 18.0.0
- ES Modules (`"type": "module"`)

## Language
- TypeScript 5.x with strict mode
- Target: ES2022
- JSX: react-jsx

## UI Framework
- **Ink** (v4.4.1) - React for CLI apps
- **React** (v18.2.0) - Component model

## Build & Dev Tools
- **tsc** - TypeScript compiler (build)
- **tsx** - TypeScript execution (dev)

## Common Commands

```bash
# Development (runs with tsx)
npm start 'command1' 'command2'
npm run dev 'command1' 'command2'

# Build (compiles to dist/)
npm run build

# Demo
npm start 'node demo/logger1.js' 'node demo/logger2.js' 'node demo/logger3.js'
```

## Output
- Compiled JS goes to `dist/`
- Generates declaration files (`.d.ts`) and source maps
- CLI entry point: `dist/index.js` (has shebang)

## Dependencies
Production: `ink`, `react`
Dev: `@types/node`, `@types/react`, `tsx`, `typescript`
