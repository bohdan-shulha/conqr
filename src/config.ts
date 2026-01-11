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

export interface ConfigFile {
  commands?: Array<string | CommandObject> | Record<string, string>;
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

function parseConfigCommands(config: ConfigFile): CommandInfo[] {
  if (!config.commands) {
    return [];
  }

  const commands: CommandInfo[] = [];
  
  // Extract global restart config from config file root
  const globalRestart = config.restart;

  if (Array.isArray(config.commands)) {
    config.commands.forEach((cmd, index) => {
      if (typeof cmd === 'string') {
        // String command: apply only global restart defaults
        commands.push({
          id: index,
          name: extractCommandName(cmd),
          command: cmd,
          restart: resolveRestartConfig(globalRestart, undefined)
        });
      } else if (typeof cmd === 'object' && cmd.name && cmd.command) {
        // CommandObject: extract per-process restart and merge with global
        const perProcessRestart = cmd.restart;
        commands.push({
          id: index,
          name: cmd.name,
          command: cmd.command,
          restart: resolveRestartConfig(globalRestart, perProcessRestart)
        });
      }
    });
  } else if (typeof config.commands === 'object') {
    // Record<string, string> format: apply only global restart defaults
    let index = 0;
    for (const [name, command] of Object.entries(config.commands)) {
      commands.push({
        id: index++,
        name,
        command: typeof command === 'string' ? command : String(command),
        restart: resolveRestartConfig(globalRestart, undefined)
      });
    }
  }

  return commands;
}

function extractCommandName(command: string): string {
  const firstWord = command.trim().split(/\s+/)[0];
  const basename = firstWord.split('/').pop();
  return basename || `cmd${Date.now()}`;
}
