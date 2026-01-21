#!/usr/bin/env node

console.log('Loader: Long Running started');
console.log('This script runs continuously and prints messages every second');

let count = 0;
const interval = setInterval(() => {
  count++;
  console.log(`[Long Running] Message #${count} - ${new Date().toLocaleTimeString()}`);
}, 1000);

process.on('SIGTERM', () => {
  console.log('[Long Running] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Long Running] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
