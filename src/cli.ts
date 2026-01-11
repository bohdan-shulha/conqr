export type RestartPolicy = 'never' | 'on-error' | 'on-exit';

export interface RestartConfig {
  policy: RestartPolicy;
  delay: number; // milliseconds
}

export interface CommandInfo {
  id: number;
  name: string;
  command: string;
  restart?: RestartConfig;
}

export function parseCommands(): CommandInfo[] {
  const args = process.argv.slice(2);

  if (args.length === 0) {
    return [];
  }

  return args.map((arg, index) => {
    const equalsIndex = arg.indexOf('=');
    let command: string;
    let name: string;

    if (equalsIndex > 0 && equalsIndex < arg.length - 1) {
      name = arg.substring(0, equalsIndex).trim();
      command = arg.substring(equalsIndex + 1).trim();

      if (name.startsWith("'") && name.endsWith("'")) {
        name = name.slice(1, -1);
      } else if (name.startsWith('"') && name.endsWith('"')) {
        name = name.slice(1, -1);
      }

      if (command.startsWith("'") && command.endsWith("'")) {
        command = command.slice(1, -1);
      } else if (command.startsWith('"') && command.endsWith('"')) {
        command = command.slice(1, -1);
      }

      if (name.length === 0) {
        name = extractCommandName(command);
      }
    } else {
      command = arg;
      name = extractCommandName(command);
    }

    return {
      id: index,
      name,
      command
    };
  });
}

function extractCommandName(command: string): string {
  const firstWord = command.trim().split(/\s+/)[0];
  const basename = firstWord.split('/').pop();
  return basename || `cmd${Date.now()}`;
}
