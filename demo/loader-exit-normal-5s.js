#!/usr/bin/env node

console.log('Loader: Exit Normal 5s started');
console.log('This script will exit normally (exit code 0) after 5 seconds');

let count = 0;
const interval = setInterval(() => {
  count++;
  console.log(`[Exit Normal 5s] Message #${count} - ${new Date().toLocaleTimeString()}`);
}, 1000);

setTimeout(() => {
  console.log('[Exit Normal 5s] Exiting normally after 5 seconds');
  clearInterval(interval);
  process.exit(0);
}, 5000);

process.on('SIGTERM', () => {
  console.log('[Exit Normal 5s] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Exit Normal 5s] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
