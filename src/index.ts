#!/usr/bin/env node

import { parseCommands } from './cli.js';
import { loadConfig } from './config.js';
import { ProcessManager } from './process-manager.js';
import { LogBuffer } from './log-buffer.js';
import { renderTUI } from './ui.jsx';

const cliCommands = parseCommands();
const configCommands = loadConfig();

let commands;
if (cliCommands.length > 0) {
  commands = cliCommands;
} else if (configCommands && configCommands.length > 0) {
  commands = configCommands;
} else {
  console.error('No commands provided. Use CLI arguments or create a conqr.json config file.');
  process.exit(1);
}

const logBuffer = new LogBuffer();
const processManager = new ProcessManager(logBuffer);

renderTUI(commands, processManager, logBuffer);
