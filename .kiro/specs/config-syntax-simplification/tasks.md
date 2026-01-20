# Implementation Plan: Config Syntax Simplification

## Overview

This implementation plan converts the conqr configuration system from supporting both array and object formats to supporting only the object/record format. Changes affect the JSON schema and config parser module.

## Tasks

- [x] 1. Update type definitions in config.ts
  - [x] 1.1 Add ExtendedCommandConfig interface for object command values
    - Define interface with required `command` and optional `restart` properties
    - _Requirements: 3.1, 3.2_
  - [x] 1.2 Add CommandValue type union (string | ExtendedCommandConfig)
    - Create type alias for command entry values
    - _Requirements: 2.1, 3.1_
  - [x] 1.3 Update ConfigFile interface to use object-only commands
    - Change `commands` type from union to `Record<string, CommandValue>`
    - _Requirements: 1.1_

- [x] 2. Update parseConfigCommands function
  - [x] 2.1 Add array format detection with error message
    - Check if commands is an array at start of function
    - Log clear error message suggesting object format
    - Return empty array or null on array detection
    - _Requirements: 1.2, 5.1, 5.2_
  - [x] 2.2 Implement simple command parsing (string values)
    - Iterate over object entries
    - For string values: use key as name, value as command
    - Apply global restart config
    - _Requirements: 2.1, 2.2, 2.3_
  - [x] 2.3 Implement extended command parsing (object values)
    - For object values: extract command property
    - Merge per-process restart with global restart
    - Skip entries missing required command property with warning
    - _Requirements: 3.1, 3.2, 3.3, 3.4_
  - [ ]* 2.4 Write property test for simple command parsing
    - **Property 3: Simple Command Parsing**
    - **Validates: Requirements 2.1, 2.2**
  - [ ]* 2.5 Write property test for extended command parsing
    - **Property 4: Extended Command Parsing**
    - **Validates: Requirements 3.1, 3.2**

- [x] 3. Checkpoint - Verify parser changes
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Update JSON schema
  - [x] 4.1 Remove array-based command definitions from schema
    - Remove `oneOf` wrapper from commands property
    - Remove array type and array item definitions
    - _Requirements: 1.3, 4.1_
  - [x] 4.2 Define object-only commands with patternProperties
    - Set commands type to object
    - Use patternProperties for string keys
    - Allow values to be string or object with command property
    - _Requirements: 4.1, 4.2, 4.3_
  - [x] 4.3 Update schema examples to show only object format
    - Remove all array-based examples
    - Add examples showing simple and extended command syntax
    - _Requirements: 4.4_

- [ ] 5. Write unit tests for config parser
  - [ ]* 5.1 Write unit tests for valid configurations
    - Test simple command parsing
    - Test extended command parsing
    - Test mixed simple and extended commands
    - Test empty commands object
    - _Requirements: 1.1, 2.1, 2.2, 3.1_
  - [ ]* 5.2 Write unit tests for restart config inheritance
    - Test global restart applies to simple commands
    - Test per-process restart overrides global
    - Test default restart when no config provided
    - _Requirements: 2.3, 3.2, 3.3_
  - [ ]* 5.3 Write unit tests for error handling
    - Test array format rejection with error message
    - Test missing command property handling
    - _Requirements: 1.2, 3.4, 5.1, 5.2_
  - [ ]* 5.4 Write property test for array format rejection
    - **Property 2: Array Format Rejection**
    - **Validates: Requirements 1.2, 5.1**
  - [ ]* 5.5 Write property test for global restart fallback
    - **Property 5: Global Restart Fallback**
    - **Validates: Requirements 2.3, 3.3**

- [x] 6. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- The existing restart config resolution logic (`resolveRestartConfig`) remains unchanged
- CLI argument parsing in `cli.ts` is not affected by this change
- Example config files (`conqr.json.example*`) should be updated to match new format
