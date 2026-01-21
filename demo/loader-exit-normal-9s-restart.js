#!/usr/bin/env node

console.log('Loader: Exit Normal 9s Restart started');
console.log('This script will exit normally (exit code 0) after 9 seconds');
console.log('Configured with restart policy "on-exit" and 2s delay');

let count = 0;
const interval = setInterval(() => {
  count++;
  console.log(`[Exit Normal 9s Restart] Message #${count} - ${new Date().toLocaleTimeString()}`);
}, 1000);

setTimeout(() => {
  console.log('[Exit Normal 9s Restart] Exiting normally after 9 seconds');
  clearInterval(interval);
  process.exit(0);
}, 9000);

process.on('SIGTERM', () => {
  console.log('[Exit Normal 9s Restart] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Exit Normal 9s Restart] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
