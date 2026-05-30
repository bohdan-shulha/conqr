#!/usr/bin/env node

import { existsSync } from 'node:fs';
import { join } from 'node:path';

const targets = [
  ['darwin', 'amd64'],
  ['darwin', 'arm64'],
  ['linux', 'amd64'],
  ['linux', 'arm64'],
  ['win32', 'amd64'],
  ['win32', 'arm64']
];

const missing = targets
  .map(([platform, arch]) => join('dist', platform === 'win32' ? `conqr-${platform}-${arch}.exe` : `conqr-${platform}-${arch}`))
  .filter((path) => !existsSync(path));

if (missing.length > 0) {
  console.error('Missing npm package binaries:');
  for (const path of missing) {
    console.error(`- ${path}`);
  }
  console.error('Run the release build before packing or publishing.');
  process.exit(1);
}
