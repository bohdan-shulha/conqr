#!/usr/bin/env node

console.log('Loader: Exit Error 7s started');
console.error('This script will exit with error code (exit code 1) after 7 seconds');

let count = 0;
const interval = setInterval(() => {
  count++;
  console.log(`[Exit Error 7s] Message #${count} - ${new Date().toLocaleTimeString()}`);
}, 1000);

setTimeout(() => {
  console.error('[Exit Error 7s] Exiting with error code after 7 seconds');
  clearInterval(interval);
  process.exit(1);
}, 7000);

process.on('SIGTERM', () => {
  console.log('[Exit Error 7s] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Exit Error 7s] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
