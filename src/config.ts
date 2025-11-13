import { readFileSync, existsSync } from 'fs';
import { join } from 'path';
import { CommandInfo } from './cli.js';

const CONFIG_FILES = ['.conqr.json', 'conqr.json'];

export interface ConfigFile {
  commands?: Array<string | { name: string; command: string }> | Record<string, string>;
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

  if (Array.isArray(config.commands)) {
    config.commands.forEach((cmd, index) => {
      if (typeof cmd === 'string') {
        commands.push({
          id: index,
          name: extractCommandName(cmd),
          command: cmd
        });
      } else if (typeof cmd === 'object' && cmd.name && cmd.command) {
        commands.push({
          id: index,
          name: cmd.name,
          command: cmd.command
        });
      }
    });
  } else if (typeof config.commands === 'object') {
    let index = 0;
    for (const [name, command] of Object.entries(config.commands)) {
      commands.push({
        id: index++,
        name,
        command: typeof command === 'string' ? command : String(command)
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
