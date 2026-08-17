#!/usr/bin/env node

import { existsSync } from 'node:fs';
import { join } from 'node:path';
import { binaryName, targets } from './binary-name.js';

const missing = targets
  .map(([platform, arch]) => join('dist', binaryName(platform, arch)))
  .filter((path) => !existsSync(path));

if (missing.length > 0) {
  console.error('Missing npm package binaries:');
  for (const path of missing) {
    console.error(`- ${path}`);
  }
  console.error('Run the release build before packing or publishing.');
  process.exit(1);
}
