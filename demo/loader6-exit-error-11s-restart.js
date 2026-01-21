#!/usr/bin/env node

// 6) A script that will exit with error after 11 sec (use with restart: 2s)
console.log('[Loader 6] Started - Will exit with ERROR in 11 seconds (restart enabled)');

let count = 0;
const interval = setInterval(() => {
  count++;
  console.log(`[Loader 6] Running... ${count}s`);
}, 1000);

setTimeout(() => {
  console.error('[Loader 6] FATAL ERROR: Process crashed! (will restart in 2s)');
  clearInterval(interval);
  process.exit(1);
}, 11000);

process.on('SIGTERM', () => {
  console.log('[Loader 6] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Loader 6] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
