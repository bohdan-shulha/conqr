#!/usr/bin/env node

console.log('Loader: Exit Error 11s Restart started');
console.error('This script will exit with error code (exit code 1) after 11 seconds');
console.log('Configured with restart policy "on-error" and 2s delay');

let count = 0;
const interval = setInterval(() => {
  count++;
  console.log(`[Exit Error 11s Restart] Message #${count} - ${new Date().toLocaleTimeString()}`);
}, 1000);

setTimeout(() => {
  console.error('[Exit Error 11s Restart] Exiting with error code after 11 seconds');
  clearInterval(interval);
  process.exit(1);
}, 11000);

process.on('SIGTERM', () => {
  console.log('[Exit Error 11s Restart] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Exit Error 11s Restart] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
