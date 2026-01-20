# Design Document: Config Syntax Simplification

## Overview

This design document describes the technical approach for simplifying the conqr configuration syntax by removing array-based command definitions and standardizing on an object/record format only. The changes affect the JSON schema (`conqr.schema.json`) and the configuration parser (`src/config.ts`).

The simplified syntax allows two forms of command definitions:
1. **Simple**: `"name": "command string"`
2. **Extended**: `"name": { "command": "command string", "restart": { ... } }`

## Architecture

The configuration system follows a straightforward flow:

```mermaid
flowchart LR
    A[conqr.json] --> B[Config Parser]
    B --> C{Validate Format}
    C -->|Object Format| D[Parse Commands]
    C -->|Array Format| E[Error: Unsupported]
    D --> F[CommandInfo[]]
    F --> G[Process Manager]
```

### Components Affected

1. **JSON Schema** (`conqr.schema.json`): Defines valid configuration structure
2. **Config Parser** (`src/config.ts`): Parses and validates configuration files
3. **Type Definitions**: Updated interfaces for the new format

## Components and Interfaces

### Updated Type Definitions

```typescript
// New interface for extended command object (value in commands record)
export interface ExtendedCommandConfig {
  command: string;
  restart?: Partial<RestartConfig>;
}

// Command value can be string or extended config
export type CommandValue = string | ExtendedCommandConfig;

// Updated config file interface - commands is now object-only
export interface ConfigFile {
  commands: Record<string, CommandValue>;
  restart?: Partial<RestartConfig>;
}
```

### Config Parser Interface

```typescript
// Existing function signature remains unchanged
export function loadConfig(): CommandInfo[] | null;

// Internal parsing function updated
function parseConfigCommands(config: ConfigFile): CommandInfo[];

// New validation function
function isValidCommandValue(value: unknown): value is CommandValue;
```

### JSON Schema Structure

The schema will be updated to:
- Remove `oneOf` with array options from `commands`
- Define `commands` as object with `patternProperties`
- Allow values to be either string or object with `command` property

## Data Models

### ConfigFile (Updated)

```typescript
interface ConfigFile {
  $schema?: string;           // Optional schema reference
  restart?: Partial<RestartConfig>;  // Global restart config
  commands: Record<string, CommandValue>;  // Object-only format
}
```

### CommandValue (New)

```typescript
type CommandValue = string | ExtendedCommandConfig;

interface ExtendedCommandConfig {
  command: string;            // Required: the command to execute
  restart?: Partial<RestartConfig>;  // Optional: per-process restart config
}
```

### CommandInfo (Unchanged)

```typescript
interface CommandInfo {
  id: number;
  name: string;
  command: string;
  restart?: RestartConfig;
}
```

### Parsing Logic

The parser transforms `Record<string, CommandValue>` into `CommandInfo[]`:

```typescript
function parseConfigCommands(config: ConfigFile): CommandInfo[] {
  const commands: CommandInfo[] = [];
  const globalRestart = config.restart;
  let index = 0;

  for (const [name, value] of Object.entries(config.commands)) {
    if (typeof value === 'string') {
      // Simple command: string value
      commands.push({
        id: index++,
        name,
        command: value,
        restart: resolveRestartConfig(globalRestart, undefined)
      });
    } else if (typeof value === 'object' && value.command) {
      // Extended command: object with command property
      commands.push({
        id: index++,
        name,
        command: value.command,
        restart: resolveRestartConfig(globalRestart, value.restart)
      });
    }
  }

  return commands;
}
```


## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Object Format Acceptance

*For any* valid configuration object where `commands` is a `Record<string, CommandValue>`, the Config_Parser SHALL successfully parse it and return a non-null `CommandInfo[]` with length equal to the number of keys in the commands object.

**Validates: Requirements 1.1**

### Property 2: Array Format Rejection

*For any* configuration object where `commands` is an array (regardless of array contents), the Config_Parser SHALL reject the configuration and return null or throw an error.

**Validates: Requirements 1.2, 5.1**

### Property 3: Simple Command Parsing

*For any* command entry where the key is a non-empty string `name` and the value is a non-empty string `cmd`, parsing SHALL produce a `CommandInfo` where `name` equals the key and `command` equals the value string.

**Validates: Requirements 2.1, 2.2**

### Property 4: Extended Command Parsing

*For any* command entry where the key is a non-empty string `name` and the value is an object with a `command` property, parsing SHALL produce a `CommandInfo` where `name` equals the key, `command` equals the object's `command` property, and `restart` reflects any per-process restart configuration merged with global defaults.

**Validates: Requirements 3.1, 3.2**

### Property 5: Global Restart Fallback

*For any* configuration with a global `restart` config and any command entry (simple or extended) without per-process restart config, the resulting `CommandInfo.restart` SHALL contain the global restart settings merged with defaults.

**Validates: Requirements 2.3, 3.3**

### Property 6: Invalid Extended Command Rejection

*For any* command entry where the value is an object missing the required `command` property, the Config_Parser SHALL skip or reject that entry (not include it in the result).

**Validates: Requirements 3.4**

## Error Handling

### Array Format Detection

When the parser detects that `commands` is an array:
1. Log a clear error message: "Array format for commands is no longer supported"
2. Suggest the fix: "Please use object format: { \"name\": \"command\" }"
3. Return `null` to indicate parsing failure

```typescript
if (Array.isArray(config.commands)) {
  console.error('Error: Array format for commands is no longer supported.');
  console.error('Please use object format: { "name": "command" } or { "name": { "command": "..." } }');
  return null;
}
```

### Invalid Extended Command

When an extended command object is missing the `command` property:
1. Log a warning with the entry name
2. Skip the invalid entry (don't include in results)
3. Continue processing other entries

```typescript
if (typeof value === 'object' && !value.command) {
  console.warn(`Warning: Command entry "${name}" is missing required "command" property, skipping.`);
  continue;
}
```

### Empty Commands Object

When `commands` is an empty object:
1. Return an empty `CommandInfo[]` array
2. The application will handle the empty state appropriately

## Testing Strategy

### Unit Tests

Unit tests should cover specific examples and edge cases:

1. **Valid simple command parsing**: Verify a simple `"api": "npm run dev"` parses correctly
2. **Valid extended command parsing**: Verify `"api": { "command": "npm run dev", "restart": {...} }` parses correctly
3. **Mixed format parsing**: Verify a config with both simple and extended commands parses correctly
4. **Empty commands object**: Verify empty `{}` returns empty array
5. **Global restart inheritance**: Verify global restart applies to simple commands
6. **Per-process restart override**: Verify per-process restart overrides global
7. **Array format rejection**: Verify array format returns null with error
8. **Missing command property**: Verify invalid extended command is skipped

### Property-Based Tests

Property-based tests should use a library like `fast-check` to verify universal properties:

**Configuration**: Minimum 100 iterations per property test

**Test Tags**: Each test should be tagged with:
- Feature: config-syntax-simplification
- Property number and description

```typescript
// Example property test structure
describe('Config Parser Properties', () => {
  it('Property 3: Simple command parsing', () => {
    fc.assert(
      fc.property(
        fc.string({ minLength: 1 }), // name
        fc.string({ minLength: 1 }), // command
        (name, command) => {
          const config = { commands: { [name]: command } };
          const result = parseConfigCommands(config);
          return result.length === 1 && 
                 result[0].name === name && 
                 result[0].command === command;
        }
      ),
      { numRuns: 100 }
    );
  });
});
```

### Test Coverage Matrix

| Property | Unit Test | Property Test |
|----------|-----------|---------------|
| Object format acceptance | ✓ | ✓ |
| Array format rejection | ✓ | ✓ |
| Simple command parsing | ✓ | ✓ |
| Extended command parsing | ✓ | ✓ |
| Global restart fallback | ✓ | ✓ |
| Invalid extended command | ✓ | ✓ |
