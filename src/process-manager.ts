import { spawn, ChildProcess } from 'child_process';
import { EventEmitter } from 'events';
import { CommandInfo } from './cli.js';
import { LogBuffer } from './log-buffer.js';

export type ProcessStatus = 'running' | 'stopped' | 'error' | 'unknown';

interface ProcessInfo extends CommandInfo {
  status: ProcessStatus;
  process: ChildProcess;
  pid?: number;
}

interface ProcessBuffer {
  stdout: string;
  stderr: string;
}

export interface LogEvent {
  processId: number;
  line: string;
  source: 'stdout' | 'stderr';
}

export interface StatusChangeEvent {
  processId: number;
  status: ProcessStatus;
}

export interface RestartStateChangeEvent {
  processId: number;
  isRestarting: boolean;
  restartCount: number;
  crashCount: number;
}

export class ProcessManager extends EventEmitter {
  private processes: Map<number, ProcessInfo>;
  private logBuffer: LogBuffer;
  private buffers: Map<number, ProcessBuffer>;
  private restartTimeouts: Map<number, NodeJS.Timeout>;
  private restartCounts: Map<number, number>;
  private crashCounts: Map<number, number>;
  private isRestartingMap: Map<number, boolean>;
  private intentionalExitProcesses: Set<ChildProcess>;

  constructor(logBuffer: LogBuffer) {
    super();
    this.processes = new Map();
    this.logBuffer = logBuffer;
    this.buffers = new Map();
    this.restartTimeouts = new Map();
    this.restartCounts = new Map();
    this.crashCounts = new Map();
    this.isRestartingMap = new Map();
    this.intentionalExitProcesses = new Set();
  }

  private detectError(line: string): boolean {
    const hasRedAnsi = /\x1b\[31m|\x1b\[91m|\x1b\[38;5;1m/.test(line);
    const errorPatterns = [
      /SyntaxError/i,
      /TypeError/i,
      /ReferenceError/i,
      /Error:/i,
      /Error\s+at/i,
      /FATAL/i,
      /CRITICAL/i,
      /failed/i,
      /failure/i,
      /cannot/i,
      /uncaught/i,
      /unhandled/i,
      /^\s+at .+\(.+:\d+:\d+\)$/,
    ];
    return hasRedAnsi || errorPatterns.some(pattern => pattern.test(line));
  }

  startCommand(commandInfo: CommandInfo): ChildProcess {
    const { id, name, command } = commandInfo;

    const proc = spawn(command, {
      shell: true,
      stdio: ['ignore', 'pipe', 'pipe'],
      detached: true
    });

    proc.stdout?.setEncoding('utf8');
    proc.stderr?.setEncoding('utf8');

    this.buffers.set(id, { stdout: '', stderr: '' });

    proc.stdout?.on('data', (data: Buffer | string) => {
      const buffer = this.buffers.get(id);
      if (!buffer) return;
      buffer.stdout += data.toString();
      const lines = buffer.stdout.split('\n');
      buffer.stdout = lines.pop() || '';
      lines.forEach(line => {
        if (line.length > 0) {
          this.logBuffer.addLog(id, line, 'stdout');
          this.emit('log', { processId: id, line, source: 'stdout' } as LogEvent);
          this.checkRecentErrors(id);
        }
      });
    });

    proc.stderr?.on('data', (data: Buffer | string) => {
      const buffer = this.buffers.get(id);
      if (!buffer) return;
      buffer.stderr += data.toString();
      const lines = buffer.stderr.split('\n');
      buffer.stderr = lines.pop() || '';
      lines.forEach(line => {
        if (line.length > 0) {
          this.logBuffer.addLog(id, line, 'stderr');
          this.emit('log', { processId: id, line, source: 'stderr' } as LogEvent);
          this.checkRecentErrors(id);
        }
      });
    });

    // Flush remaining buffer data when streams end
    const flushBuffer = (source: 'stdout' | 'stderr') => {
      const buffer = this.buffers.get(id);
      if (!buffer) return;
      const bufferContent = source === 'stdout' ? buffer.stdout : buffer.stderr;
      if (bufferContent.trim().length > 0) {
        this.logBuffer.addLog(id, bufferContent.trim(), source);
        this.emit('log', { processId: id, line: bufferContent.trim(), source } as LogEvent);
        this.checkRecentErrors(id);
      }
      if (source === 'stdout') {
        buffer.stdout = '';
      } else {
        buffer.stderr = '';
      }
    };

    proc.stdout?.on('end', () => {
      flushBuffer('stdout');
    });

    proc.stderr?.on('end', () => {
      flushBuffer('stderr');
    });

    proc.on('error', (err: Error) => {
      this.logBuffer.addLog(id, `Process error: ${err.message}`, 'stderr', true);
      const current = this.processes.get(id);
      if (current && current.process === proc) {
        current.status = 'error';
        current.pid = proc.pid;
        this.emit('status-change', { processId: id, status: 'error' } as StatusChangeEvent);
      }
    });

    proc.on('exit', (code: number | null) => {
      const wasIntentionalExit = this.intentionalExitProcesses.has(proc);
      this.intentionalExitProcesses.delete(proc);
      const hasNonZeroExitCode = typeof code === 'number' && code !== 0;

      const procInfo = this.processes.get(id);
      if (procInfo && procInfo.process === proc) {
        // Flush any remaining buffer data before logging exit message
        const buffer = this.buffers.get(id);
        if (buffer) {
          if (buffer.stdout.trim().length > 0) {
            this.logBuffer.addLog(id, buffer.stdout.trim(), 'stdout');
            this.emit('log', { processId: id, line: buffer.stdout.trim(), source: 'stdout' } as LogEvent);
            this.checkRecentErrors(id);
            buffer.stdout = '';
          }
          if (buffer.stderr.trim().length > 0) {
            this.logBuffer.addLog(id, buffer.stderr.trim(), 'stderr');
            this.emit('log', { processId: id, line: buffer.stderr.trim(), source: 'stderr' } as LogEvent);
            this.checkRecentErrors(id);
            buffer.stderr = '';
          }
        }

        procInfo.status = 'stopped';
        procInfo.pid = proc.pid;
        this.emit('status-change', { processId: id, status: 'stopped' } as StatusChangeEvent);

        // Increment crash count for non-zero exits
        if (hasNonZeroExitCode && !wasIntentionalExit) {
          const currentCrashCount = this.crashCounts.get(id) || 0;
          this.crashCounts.set(id, currentCrashCount + 1);
          this.emitRestartStateChange(id);
        }

        // Check restart policy and schedule restart if needed (Requirements 1.4, 1.5, 1.6)
        const restartConfig = procInfo.restart;
        let willRestart = false;
        if (restartConfig && !wasIntentionalExit) {
          const { policy, delay } = restartConfig;

          if (policy === 'on-exit') {
            // Requirement 1.6: Restart whenever process exits, regardless of exit code
            this.scheduleRestart(id, delay, code);
            willRestart = true;
          } else if (policy === 'on-error' && hasNonZeroExitCode) {
            // Requirement 1.5: Restart only when exit code is non-zero
            this.scheduleRestart(id, delay, code);
            willRestart = true;
          }
          // Requirement 1.4: If policy is 'never', do nothing (existing behavior)
        }

        // Log exit code for non-restartable processes or when restart won't happen
        if (!willRestart) {
          const exitEmoji = code === 0 ? '•' : '×';
          const exitCodeStr = code === null ? 'null' : code.toString();
          const exitMessage = `${exitEmoji} Process exited with code ${exitCodeStr}`.trim();
          this.logBuffer.addLog(id, exitMessage, 'stdout', true);
          this.emit('log', { processId: id, line: exitMessage, source: 'stdout' } as LogEvent);
        }
      }
    });

    const startMessage = '› Service starting';
    this.logBuffer.addLog(id, startMessage, 'stdout', true);
    this.emit('log', { processId: id, line: startMessage, source: 'stdout' } as LogEvent);

    const procInfo: ProcessInfo = { ...commandInfo, status: 'running', process: proc, pid: proc.pid };
    this.processes.set(id, procInfo);
    this.emit('status-change', { processId: id, status: 'running' } as StatusChangeEvent);

    return proc;
  }

  startAll(commands: CommandInfo[]): void {
    commands.forEach(cmd => this.startCommand(cmd));
  }

  getStatus(processId: number): ProcessStatus {
    const procInfo = this.processes.get(processId);
    return procInfo ? procInfo.status : 'unknown';
  }

  getAllStatuses(): Map<number, ProcessStatus> {
    const statuses = new Map<number, ProcessStatus>();
    this.processes.forEach((procInfo, id) => {
      statuses.set(id, procInfo.status);
    });
    return statuses;
  }

  private updateStatusToError(processId: number): void {
    const procInfo = this.processes.get(processId);
    if (procInfo && procInfo.status === 'running') {
      procInfo.status = 'error';
      this.emit('status-change', { processId, status: 'error' } as StatusChangeEvent);
    }
  }

  private checkRecentErrors(processId: number): void {
    const procInfo = this.processes.get(processId);
    if (!procInfo || procInfo.status === 'stopped' || procInfo.status === 'unknown') {
      return;
    }

    const logs = this.logBuffer.getLogs(processId);
    const recentLogs = logs.slice(-10);

    const hasRecentError = recentLogs.some(log => this.detectError(log.line));

    if (hasRecentError && procInfo.status === 'running') {
      procInfo.status = 'error';
      this.emit('status-change', { processId, status: 'error' } as StatusChangeEvent);
    } else if (!hasRecentError && procInfo.status === 'error' && procInfo.process.exitCode === null && !procInfo.process.killed) {
      procInfo.status = 'running';
      this.emit('status-change', { processId, status: 'running' } as StatusChangeEvent);
    }
  }

  private killProcess(procInfo: ProcessInfo, signal: NodeJS.Signals): void {
    if (!procInfo.process || procInfo.process.killed) {
      return;
    }

    try {
      this.intentionalExitProcesses.add(procInfo.process);
      const pid = procInfo.process.pid;
      if (!pid) {
        return;
      }

      if (process.platform !== 'win32') {
        try {
          process.kill(-pid, signal);
        } catch {
          procInfo.process.kill(signal);
        }
      } else {
        try {
          spawn('taskkill', ['/pid', pid.toString(), '/t', '/f'], { stdio: 'ignore' });
        } catch {
          procInfo.process.kill(signal);
        }
      }
    } catch {
    }
  }

  async killAll(): Promise<void> {
    // Clear all pending restart timeouts to prevent restarts during shutdown
    // This ensures processes don't auto-restart while the application is shutting down
    for (const timeout of this.restartTimeouts.values()) {
      clearTimeout(timeout);
    }
    this.restartTimeouts.clear();
    // Clear restart states
    for (const processId of this.isRestartingMap.keys()) {
      this.isRestartingMap.set(processId, false);
      this.emitRestartStateChange(processId);
    }

    const killPromises: Promise<void>[] = [];

    this.processes.forEach((procInfo) => {
      if (!procInfo.process || procInfo.process.killed) {
        return;
      }

      const promise = new Promise<void>((resolve) => {
        let resolved = false;
        const timeout = setTimeout(() => {
          if (!resolved) {
            resolved = true;
            resolve();
          }
        }, 1000);

        const onExit = () => {
          if (!resolved) {
            resolved = true;
            clearTimeout(timeout);
            resolve();
          }
        };

        procInfo.process.once('exit', onExit);
        this.killProcess(procInfo, 'SIGTERM');
      });
      killPromises.push(promise);
    });

    await Promise.all(killPromises);

    const forceKillPromises: Promise<void>[] = [];
    this.processes.forEach((procInfo) => {
      if (!procInfo.process || procInfo.process.killed) {
        return;
      }

      this.killProcess(procInfo, 'SIGKILL');

      const promise = new Promise<void>((resolve) => {
        let resolved = false;
        const timeout = setTimeout(() => {
          if (!resolved) {
            resolved = true;
            resolve();
          }
        }, 500);

        const onExit = () => {
          if (!resolved) {
            resolved = true;
            clearTimeout(timeout);
            resolve();
          }
        };

        procInfo.process.once('exit', onExit);
      });
      forceKillPromises.push(promise);
    });

    await Promise.all(forceKillPromises);
  }

  private async killOne(procInfo: ProcessInfo): Promise<void> {
    if (!procInfo.process || procInfo.process.killed) {
      return;
    }

    await new Promise<void>((resolve) => {
      let resolved = false;
      const timeout = setTimeout(() => {
        if (!resolved) {
          resolved = true;
          resolve();
        }
      }, 1000);

      const onExit = () => {
        if (!resolved) {
          resolved = true;
          clearTimeout(timeout);
          resolve();
        }
      };

      procInfo.process.once('exit', onExit);
      this.killProcess(procInfo, 'SIGTERM');
    });

    if (!procInfo.process.killed) {
      await new Promise<void>((resolve) => {
        let resolved = false;
        const timeout = setTimeout(() => {
          if (!resolved) {
            resolved = true;
            resolve();
          }
        }, 500);

        const onExit = () => {
          if (!resolved) {
            resolved = true;
            clearTimeout(timeout);
            resolve();
          }
        };

        procInfo.process.once('exit', onExit);
        this.killProcess(procInfo, 'SIGKILL');
      });
    }
  }

  async restart(processId: number, isManual: boolean = false): Promise<void> {
    // Clear any pending auto-restart timeout for this process (Requirement 1.4)
    // This prevents duplicate restarts when user manually restarts a process
    // that already has an auto-restart scheduled
    const pendingTimeout = this.restartTimeouts.get(processId);
    if (pendingTimeout) {
      clearTimeout(pendingTimeout);
      this.restartTimeouts.delete(processId);
    }

    const procInfo = this.processes.get(processId);
    if (!procInfo) {
      return;
    }

    // Log manual restart message immediately when restart is initiated
    if (isManual) {
      const restartMessage = '› Restart initiated, SIGTERM signal sent';
      this.logBuffer.addLog(processId, restartMessage, 'stdout', true);
      this.emit('log', { processId, line: restartMessage, source: 'stdout' } as LogEvent);
    }

    if (procInfo.process && !procInfo.process.killed) {
      await this.killOne(procInfo);
    }

    // Increment restart count
    const currentRestartCount = this.restartCounts.get(processId) || 0;
    this.restartCounts.set(processId, currentRestartCount + 1);

    // Set restarting state to false since we're about to start the new process
    this.isRestartingMap.set(processId, false);

    this.startCommand({
      id: procInfo.id,
      name: procInfo.name,
      command: procInfo.command,
      restart: procInfo.restart
    });

    // Emit restart state change after restart completes
    this.emitRestartStateChange(processId);
  }

  private scheduleRestart(processId: number, delay: number, exitCode?: number | null): void {
    // Clear any existing timeout for this process to prevent duplicate restarts
    const existingTimeout = this.restartTimeouts.get(processId);
    if (existingTimeout) {
      clearTimeout(existingTimeout);
    }

    // Get process info for logging
    const procInfo = this.processes.get(processId);
    const processName = procInfo?.name ?? `Process ${processId}`;

    // Log restart message with delay info (Requirement 3.1)
    const delaySeconds = (delay / 1000).toFixed(1);
    let restartMessage: string;

    if (exitCode !== undefined) {
      // Combined exit and restart message for auto-restarts
      const exitEmoji = exitCode === 0 ? '•' : '×';
      const exitCodeStr = exitCode === null ? 'null' : exitCode.toString();
      restartMessage = `${exitEmoji} Process exited with code ${exitCodeStr} › ${delaySeconds}s`.trim();
    } else {
      // Manual restart message (shouldn't happen in practice, but keeping for safety)
      restartMessage = `› ${processName} ${delaySeconds}s`.trim();
    }

    this.logBuffer.addLog(processId, restartMessage, 'stdout', true);
    this.emit('log', { processId, line: restartMessage, source: 'stdout' } as LogEvent);

    // Set restarting state to true
    this.isRestartingMap.set(processId, true);
    this.emitRestartStateChange(processId);

    // Schedule restart via setTimeout (Requirement 1.7)
    const timeout = setTimeout(() => {
      this.restartTimeouts.delete(processId);
      this.restart(processId, false); // false = automatic restart
    }, delay);

    // Store the timeout in the map
    this.restartTimeouts.set(processId, timeout);
  }

  private emitRestartStateChange(processId: number): void {
    const isRestarting = this.isRestartingMap.get(processId) || false;
    const restartCount = this.restartCounts.get(processId) || 0;
    const crashCount = this.crashCounts.get(processId) || 0;
    this.emit('restart-state-change', {
      processId,
      isRestarting,
      restartCount,
      crashCount
    } as RestartStateChangeEvent);
  }

  isRestarting(processId: number): boolean {
    return this.isRestartingMap.get(processId) || false;
  }

  getRestartCount(processId: number): number {
    return this.restartCounts.get(processId) || 0;
  }

  getCrashCount(processId: number): number {
    return this.crashCounts.get(processId) || 0;
  }
}
