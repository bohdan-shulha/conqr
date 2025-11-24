#!/usr/bin/env node

console.log("Logger 3 started");
console.log("This process runs continuously until stopped");

let count = 0;
const interval = setInterval(() => {
  count++;
  const messages = [
    "Processing data...",
    "Fetching resources...",
    "Updating cache...",
    "Validating input...",
    "Saving results...",
  ];
  const message = messages[count % messages.length];
  console.log(`[Logger 3] ${message} (iteration ${count})`);
}, 350);

process.on("SIGTERM", () => {
  console.log("[Logger 3] Received SIGTERM, shutting down gracefully...");
  clearInterval(interval);
  process.exit(0);
});

process.on("SIGINT", () => {
  console.log("[Logger 3] Received SIGINT, shutting down gracefully...");
  clearInterval(interval);
  process.exit(0);
});
