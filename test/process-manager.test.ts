import assert from 'node:assert/strict';
import test from 'node:test';
import { setTimeout as delay } from 'node:timers/promises';
import { LogBuffer } from '../src/log-buffer.js';
import { ProcessManager } from '../src/process-manager.js';

function buildCommand(code: string): string {
  return `${JSON.stringify(process.execPath)} -e ${JSON.stringify(code)}`;
}

async function waitUntil(predicate: () => boolean, timeoutMs: number = 1000): Promise<void> {
  const deadline = Date.now() + timeoutMs;

  while (Date.now() < deadline) {
    if (predicate()) {
      return;
    }

    await delay(10);
  }

  assert.fail(`Timed out after ${timeoutMs}ms`);
}

function countStartLogs(logBuffer: LogBuffer, processId: number): number {
  return logBuffer.getLogs(processId).filter(entry => entry.line === '› Service starting').length;
}

test('scheduled restart does not restart a process that is already running again', async (t) => {
  const logBuffer = new LogBuffer();
  const processManager = new ProcessManager(logBuffer);

  t.after(async () => {
    await processManager.killAll();
  });

  processManager.startCommand({
    id: 1,
    name: 'steady',
    command: buildCommand('setInterval(() => {}, 1000);')
  });

  await waitUntil(() => processManager.getStatus(1) === 'running' && countStartLogs(logBuffer, 1) === 1);

  (processManager as any).scheduleRestart(1, 0, 0, (processManager as any).processes.get(1).runId);

  await delay(50);

  assert.equal(countStartLogs(logBuffer, 1), 1);
  assert.equal(processManager.getStatus(1), 'running');
  assert.equal(processManager.isRestarting(1), false);
});

test('concurrent manual restarts only start one replacement process', async (t) => {
  const logBuffer = new LogBuffer();
  const processManager = new ProcessManager(logBuffer);

  t.after(async () => {
    await processManager.killAll();
  });

  processManager.startCommand({
    id: 1,
    name: 'short-lived',
    command: buildCommand('setTimeout(() => process.exit(0), 200);')
  });

  await waitUntil(() => processManager.getStatus(1) === 'running');
  await waitUntil(() => processManager.getStatus(1) === 'stopped', 1500);

  await Promise.all([
    processManager.restart(1, true),
    processManager.restart(1, true)
  ]);

  await waitUntil(() => countStartLogs(logBuffer, 1) >= 2);
  await delay(50);

  assert.equal(countStartLogs(logBuffer, 1), 2);
  assert.equal(processManager.getRestartCount(1), 1);
});