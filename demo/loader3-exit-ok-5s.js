#!/usr/bin/env node

// 3) A script that will exit normally after 5 sec
console.log('[Loader 3] Started - Will exit normally in 5 seconds');

let count = 0;
const interval = setInterval(() => {
  count++;
  console.log(`[Loader 3] Running... ${count}s`);
}, 1000);

setTimeout(() => {
  console.log('[Loader 3] Completed successfully!');
  clearInterval(interval);
  process.exit(0);
}, 5000);

process.on('SIGTERM', () => {
  console.log('[Loader 3] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Loader 3] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
