# Requirements Document

## Introduction

This feature adds configurable automatic restart behavior for processes managed by conqr. Currently, conqr supports manual process restart via keyboard shortcut, but users need the ability to configure processes to automatically restart when they crash or exit. This is essential for development workflows where processes like dev servers or watchers should recover automatically from transient failures.

## Glossary

- **Process_Manager**: The component responsible for spawning, monitoring, and managing the lifecycle of child processes
- **Restart_Policy**: A string value that defines when a process should be automatically restarted: `"never"` (no auto-restart), `"on-error"` (restart only on non-zero exit), or `"on-exit"` (restart on any exit)
- **Restart_Delay**: The time in milliseconds to wait before attempting to restart a process

## Requirements

### Requirement 1: Global Restart Configuration

**User Story:** As a user, I want to configure default restart behavior for all processes, so that I don't have to repeat configuration for each command.

#### Acceptance Criteria

1. WHEN a config file contains a `restart` object at the root level, THE Config_Loader SHALL apply those settings as defaults for all commands
2. WHEN no restart configuration is provided, THE Process_Manager SHALL NOT automatically restart any processes (equivalent to `policy: "never"`)
3. THE Config_Loader SHALL support the following restart options: `policy` (string) and `delay` (number)
4. WHEN `policy` is set to `"never"`, THE Process_Manager SHALL NOT automatically restart the process
5. WHEN `policy` is set to `"on-error"`, THE Process_Manager SHALL restart the process only when it exits with a non-zero exit code
6. WHEN `policy` is set to `"on-exit"`, THE Process_Manager SHALL restart the process whenever it exits, regardless of exit code
7. WHEN `delay` is specified, THE Process_Manager SHALL wait the specified milliseconds before attempting a restart

### Requirement 2: Per-Process Restart Configuration

**User Story:** As a user, I want to configure restart behavior for individual processes, so that I can have different restart policies for different commands.

#### Acceptance Criteria

1. WHEN a command object contains a `restart` property, THE Config_Loader SHALL use those settings for that specific command
2. WHEN a command has per-process restart settings, THE Config_Loader SHALL merge them with global defaults, with per-process settings taking precedence
3. WHEN using the object command format, THE Config_Loader SHALL accept restart configuration alongside `name` and `command` properties
4. WHEN using the simple string command format, THE Process_Manager SHALL apply only global restart defaults

### Requirement 3: Restart Status Feedback

**User Story:** As a user, I want to see when processes are restarting, so that I can understand what's happening with my processes.

#### Acceptance Criteria

1. WHEN a process is about to restart, THE Process_Manager SHALL log a message indicating the restart and delay
2. WHEN a process successfully restarts, THE Process_Manager SHALL emit a status change event to update the UI
3. THE Log_Buffer SHALL include restart-related messages in the process log stream

### Requirement 4: Schema Validation

**User Story:** As a user, I want my restart configuration validated, so that I catch configuration errors early.

#### Acceptance Criteria

1. THE JSON_Schema SHALL define the restart configuration object with proper types and constraints
2. WHEN `policy` is provided, THE JSON_Schema SHALL validate it as one of: `"never"`, `"on-error"`, `"on-exit"`
3. WHEN `delay` is provided, THE JSON_Schema SHALL validate it as a non-negative integer
4. THE JSON_Schema SHALL include examples demonstrating restart configuration usage
