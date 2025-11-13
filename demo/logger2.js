#!/usr/bin/env node

console.log('Logger 2 started');
console.error('This process outputs to stderr occasionally');

let count = 0;
const interval = setInterval(() => {
  count++;
  if (count % 3 === 0) {
    console.error(`[Logger 2] ERROR: Something happened at ${new Date().toLocaleTimeString()}`);
  } else {
    console.log(`[Logger 2] Info message #${count}`);
  }

  if (count >= 15) {
    console.log('[Logger 2] Completed successfully');
    clearInterval(interval);
    process.exit(0);
  }
}, 1500);

process.on('SIGTERM', () => {
  console.log('[Logger 2] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Logger 2] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
