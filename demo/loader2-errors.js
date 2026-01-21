#!/usr/bin/env node

// 2) A script emitting errors (but keeps running)
console.log('[Loader 2] Started - Error emitting process');

let count = 0;
const interval = setInterval(() => {
  count++;
  if (count % 2 === 0) {
    console.error(`[Loader 2] ERROR: Something went wrong at ${new Date().toLocaleTimeString()}`);
  } else {
    console.log(`[Loader 2] Normal message #${count}`);
  }
}, 1200);

process.on('SIGTERM', () => {
  console.log('[Loader 2] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Loader 2] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
