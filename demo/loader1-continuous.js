#!/usr/bin/env node

// 1) Long-running script constantly printing messages
console.log('[Loader 1] Started - Long-running continuous process');

let count = 0;
const interval = setInterval(() => {
  count++;
  console.log(`[Loader 1] Message #${count} - ${new Date().toLocaleTimeString()}`);
}, 800);

process.on('SIGTERM', () => {
  console.log('[Loader 1] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Loader 1] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
