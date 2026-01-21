#!/usr/bin/env node

// 4) A script that will exit with error after 7 sec
console.log('[Loader 4] Started - Will exit with ERROR in 7 seconds');

let count = 0;
const interval = setInterval(() => {
  count++;
  console.log(`[Loader 4] Running... ${count}s`);
}, 1000);

setTimeout(() => {
  console.error('[Loader 4] FATAL ERROR: Process crashed!');
  clearInterval(interval);
  process.exit(1);
}, 7000);

process.on('SIGTERM', () => {
  console.log('[Loader 4] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Loader 4] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
