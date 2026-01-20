import { readFileSync, existsSync } from 'fs';
import { join } from 'path';
import { CommandInfo, RestartConfig } from './cli.js';

const CONFIG_FILES = ['.conqr.json', 'conqr.json'];

/**
 * Default restart configuration used when no restart settings are provided.
 * Policy 'never' means processes won't auto-restart by default.
 */
export const DEFAULT_RESTART_CONFIG: RestartConfig = {
  policy: 'never',
  delay: 1000 // 1 second default delay
};

/**
 * Resolves restart configuration by merging defaults, global config, and per-process config.
 * Precedence (highest to lowest): per-process → global → defaults
 * 
 * @param global - Global restart config from config file root
 * @param perProcess - Per-process restart config from command object
 * @returns Complete RestartConfig with all fields populated
 */
export function resolveRestartConfig(
  global?: Partial<RestartConfig>,
  perProcess?: Partial<RestartConfig>
): RestartConfig {
  return {
    ...DEFAULT_RESTART_CONFIG,
    ...global,
    ...perProcess
  };
}

export interface CommandObject {
  name: string;
  command: string;
  restart?: Partial<RestartConfig>;
}

/**
 * Extended command configuration for object command values.
 * Used when a command entry value is an object instead of a simple string.
 */
export interface ExtendedCommandConfig {
  /** The command string to execute (required) */
  command: string;
  /** Optional per-process restart configuration */
  restart?: Partial<RestartConfig>;
}

/**
 * Command value can be a simple string (the command to execute)
 * or an extended config object with command and optional restart settings.
 */
export type CommandValue = string | ExtendedCommandConfig;

export interface ConfigFile {
  commands: Record<string, CommandValue>;
  restart?: Partial<RestartConfig>;
}

export function loadConfig(): CommandInfo[] | null {
  const cwd = process.cwd();

  for (const configFile of CONFIG_FILES) {
    const configPath = join(cwd, configFile);
    if (existsSync(configPath)) {
      try {
        const content = readFileSync(configPath, 'utf-8');
        const config: ConfigFile = JSON.parse(content);
        return parseConfigCommands(config);
      } catch (err) {
        console.error(`Error reading config file ${configFile}:`, err);
        return null;
      }
    }
  }

  return null;
}

function parseConfigCommands(config: ConfigFile): CommandInfo[] | null {
  if (!config.commands) {
    return [];
  }

  // Check for array format - no longer supported (Requirements 1.2, 5.1, 5.2)
  if (Array.isArray(config.commands)) {
    console.error('Error: Array format for commands is no longer supported.');
    console.error('Please use object format: { "name": "command" } or { "name": { "command": "..." } }');
    return null;
  }

  const commands: CommandInfo[] = [];
  
  // Extract global restart config from config file root
  const globalRestart = config.restart;

  // Record<string, CommandValue> format: process object entries
  let index = 0;
  for (const [name, value] of Object.entries(config.commands)) {
    if (typeof value === 'string') {
      // Simple command: string value (Requirements 2.1, 2.2, 2.3)
      // - Key is used as the process display name
      // - Value is the command to execute
      // - Global restart config is applied
      commands.push({
        id: index++,
        name,
        command: value,
        restart: resolveRestartConfig(globalRestart, undefined)
      });
    } else if (typeof value === 'object' && value !== null) {
      // Extended command: object value (Requirements 3.1, 3.2, 3.3, 3.4)
      if (value.command) {
        // Valid extended command with required 'command' property
        // - Key is used as the process display name
        // - value.command is the command to execute
        // - Per-process restart config is merged with global restart config
        commands.push({
          id: index++,
          name,
          command: value.command,
          restart: resolveRestartConfig(globalRestart, value.restart)
        });
      } else {
        // Invalid extended command: missing required 'command' property (Requirement 3.4)
        console.warn(`Warning: Command entry "${name}" is missing required "command" property, skipping.`);
        continue;
      }
    }
  }

  return commands;
}
