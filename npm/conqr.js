#!/usr/bin/env node

import { existsSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn } from 'node:child_process';

const __dirname = dirname(fileURLToPath(import.meta.url));
const platform = process.platform;
const arch = process.arch;

const binaryName = platform === 'win32'
  ? `conqr-${platform}-${arch}.exe`
  : `conqr-${platform}-${arch}`;
const binaryPath = join(__dirname, '..', 'dist', binaryName);

if (!existsSync(binaryPath)) {
  console.error(`conqr: no bundled binary for ${platform}/${arch}`);
  console.error(`Expected: ${binaryPath}`);
  process.exit(1);
}

const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: 'inherit'
});

child.on('error', (error) => {
  console.error(`conqr: failed to start bundled binary: ${error.message}`);
  process.exit(1);
});

child.on('exit', (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 0);
});
