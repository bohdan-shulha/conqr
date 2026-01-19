# Implementation Plan: Process Restart Configuration

## Overview

This plan implements automatic restart configuration for conqr processes. The implementation extends existing interfaces and adds restart logic to the ProcessManager, with schema updates for validation.

## Tasks

- [x] 1. Define restart types and extend interfaces
  - [x] 1.1 Add `RestartConfig` interface and `RestartPolicy` type to `src/cli.ts`
    - Define `RestartPolicy = 'never' | 'on-error' | 'on-exit'`
    - Define `RestartConfig` with `policy` and `delay` fields
    - Add optional `restart?: RestartConfig` to `CommandInfo`
    - _Requirements: 1.3_
  
  - [x] 1.2 Extend config interfaces in `src/config.ts`
    - Add `restart?: Partial<RestartConfig>` to `ConfigFile` interface
    - Create `CommandObject` interface with `name`, `command`, and optional `restart`
    - Update array item type to support `CommandObject`
    - _Requirements: 1.1, 2.3_

- [x] 2. Implement config parsing with restart support
  - [x] 2.1 Add config resolution helper function
    - Create `resolveRestartConfig(global?, perProcess?)` function
    - Implement merge logic: defaults → global → per-process
    - Export `DEFAULT_RESTART_CONFIG` constant
    - _Requirements: 2.2_
  
  - [x] 2.2 Update `parseConfigCommands` to handle restart config
    - Parse global `restart` from config file
    - Parse per-process `restart` from command objects
    - Call `resolveRestartConfig` for each command
    - Attach resolved restart config to `CommandInfo`
    - _Requirements: 1.1, 2.1, 2.4_
  
  - [~]* 2.3 Write property test for config resolution precedence
    - **Property 1: Config Resolution Precedence**
    - **Validates: Requirements 1.1, 2.1, 2.2, 2.4**

- [x] 3. Implement restart logic in ProcessManager
  - [x] 3.1 Add restart tracking state to ProcessManager
    - Add `restartTimeouts: Map<number, NodeJS.Timeout>` field
    - Initialize in constructor
    - _Requirements: 1.7_
  
  - [x] 3.2 Implement `scheduleRestart` method
    - Accept processId and delay parameters
    - Clear any existing timeout for the process
    - Log restart message with delay info
    - Schedule restart via setTimeout
    - Call existing `restart` method when timeout fires
    - _Requirements: 1.7, 3.1_
  
  - [x] 3.3 Update exit handler to check restart policy
    - Get restart config from process info
    - If policy is `"never"`, do nothing (existing behavior)
    - If policy is `"on-error"` and exit code != 0, schedule restart
    - If policy is `"on-exit"`, always schedule restart
    - _Requirements: 1.4, 1.5, 1.6_
  
  - [~]* 3.4 Write property test for on-error policy
    - **Property 2: On-Error Policy Behavior**
    - **Validates: Requirements 1.5**
  
  - [~]* 3.5 Write property test for on-exit policy
    - **Property 3: On-Exit Policy Behavior**
    - **Validates: Requirements 1.6**

- [x] 4. Handle restart edge cases
  - [x] 4.1 Clear pending restarts on manual restart
    - In `restart` method, clear any pending timeout for the process
    - _Requirements: 1.4_
  
  - [x] 4.2 Clear all pending restarts on killAll
    - In `killAll` method, clear all restart timeouts
    - Prevent restarts during shutdown
    - _Requirements: 1.4_

- [x] 5. Update JSON schema
  - [x] 5.1 Add restart schema definition to `conqr.schema.json`
    - Define `RestartConfig` schema with policy enum and delay integer
    - Add `restart` property to root config object
    - Add `restart` property to command object definition
    - _Requirements: 4.1, 4.2, 4.3_
  
  - [x] 5.2 Add restart configuration examples to schema
    - Add example with global restart config
    - Add example with per-process restart override
    - _Requirements: 4.4_

- [x] 6. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- The implementation uses TypeScript matching the existing codebase
- Property tests should use fast-check library
- Each task references specific requirements for traceability
