# Requirements Document

## Introduction

This document specifies the requirements for simplifying the conqr configuration syntax. The goal is to streamline the `commands` configuration by removing array syntax support and standardizing on an object/record format only. This simplification improves readability and reduces cognitive overhead when writing configuration files.

## Glossary

- **Config_Parser**: The module responsible for reading and parsing conqr configuration files
- **Command_Entry**: A single command definition within the commands object
- **Simple_Command**: A command entry where the value is just a string (the command to execute)
- **Extended_Command**: A command entry where the value is an object containing `command` and optional `restart` configuration
- **Restart_Config**: Configuration object specifying automatic restart behavior for a process
- **JSON_Schema**: The schema file that validates conqr configuration files

## Requirements

### Requirement 1: Object-Only Commands Format

**User Story:** As a user, I want to define commands using only the object/record format, so that configuration files are consistent and easier to read.

#### Acceptance Criteria

1. WHEN a configuration file is loaded, THE Config_Parser SHALL only accept commands defined as an object/record where keys are process names
2. WHEN a configuration file contains commands as an array, THE Config_Parser SHALL reject the configuration and report an error
3. THE JSON_Schema SHALL define commands as an object type only, removing all array-based definitions

### Requirement 2: Simple Command Syntax

**User Story:** As a user, I want to define simple commands using just a string value, so that basic configurations remain concise.

#### Acceptance Criteria

1. WHEN a Command_Entry value is a string, THE Config_Parser SHALL interpret the string as the command to execute
2. WHEN a Command_Entry value is a string, THE Config_Parser SHALL use the object key as the process display name
3. WHEN a Simple_Command is parsed, THE Config_Parser SHALL apply global restart configuration if defined

### Requirement 3: Extended Command Syntax

**User Story:** As a user, I want to define commands with additional options using an object value, so that I can configure per-process restart behavior.

#### Acceptance Criteria

1. WHEN a Command_Entry value is an object, THE Config_Parser SHALL require a `command` property containing the command string
2. WHEN a Command_Entry value is an object with a `restart` property, THE Config_Parser SHALL apply the per-process restart configuration
3. WHEN a Command_Entry value is an object without a `restart` property, THE Config_Parser SHALL apply global restart configuration if defined
4. IF a Command_Entry object is missing the required `command` property, THEN THE Config_Parser SHALL reject the entry and report an error

### Requirement 4: Schema Validation

**User Story:** As a user, I want IDE validation for my configuration files, so that I can catch errors before running conqr.

#### Acceptance Criteria

1. THE JSON_Schema SHALL validate that commands is an object with string keys
2. THE JSON_Schema SHALL validate that command values are either strings or objects with required `command` property
3. THE JSON_Schema SHALL validate that Extended_Command objects may optionally include a `restart` property
4. THE JSON_Schema SHALL include updated examples showing only the object/record format

### Requirement 5: Backward Compatibility Error Messaging

**User Story:** As a user migrating from the old format, I want clear error messages when using deprecated array syntax, so that I understand how to update my configuration.

#### Acceptance Criteria

1. WHEN a configuration file uses array syntax for commands, THE Config_Parser SHALL provide a clear error message indicating array format is no longer supported
2. WHEN reporting the array format error, THE Config_Parser SHALL suggest using the object/record format instead
