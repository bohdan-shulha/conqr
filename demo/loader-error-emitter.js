#!/usr/bin/env node

console.log('Loader: Error Emitter started');
console.error('This script continuously prints messages and emits errors to stderr');

let count = 0;
const interval = setInterval(() => {
  count++;
  if (count % 3 === 0) {
    console.error(`[Error Emitter] ERROR: Something went wrong at ${new Date().toLocaleTimeString()}`);
  } else {
    console.log(`[Error Emitter] Info message #${count} - ${new Date().toLocaleTimeString()}`);
  }
}, 1000);

process.on('SIGTERM', () => {
  console.log('[Error Emitter] Received SIGTERM, shutting down...');
  clearInterval(interval);
  process.exit(0);
});

process.on('SIGINT', () => {
  console.log('[Error Emitter] Received SIGINT, shutting down...');
  clearInterval(interval);
  process.exit(0);
});
