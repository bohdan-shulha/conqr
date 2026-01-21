#!/usr/bin/env node

// 5) A script that will exit normally after 9 sec (use with restart: 2s)
console.log('[Loader 5] Started - Will exit normally in 9 seconds (restart enabled)');

let count = 0;
const interval = setInterval(() => {
  count++;
  console.log(`[Loader 5] Running... ${count}s`);
}, 1000);

setTimeout(() => {
  console.log('[Loader 5] Completed successfully! (will restart in 2s)');
  clearInterval(interval);
  process.exit(0);
}, 9000);

process.on('SIGTERM', () => {
  console.log('[Loader 5] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Loader 5] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
