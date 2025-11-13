#!/usr/bin/env node

console.log('Logger 1 started');
console.log('This is a demo process that logs messages');

let count = 0;
const interval = setInterval(() => {
  count++;
  console.log(`[Logger 1] Message #${count} - ${new Date().toLocaleTimeString()}`);

  if (count >= 20) {
    console.log('[Logger 1] Finished');
    clearInterval(interval);
    process.exit(0);
  }
}, 1000);

process.on('SIGTERM', () => {
  console.log('[Logger 1] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Logger 1] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
