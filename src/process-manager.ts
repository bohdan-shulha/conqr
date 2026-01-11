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

export class ProcessManager extends EventEmitter {
  private processes: Map<number, ProcessInfo>;
  private logBuffer: LogBuffer;
  private buffers: Map<number, ProcessBuffer>;
  private restartTimeouts: Map<number, NodeJS.Timeout>;

  constructor(logBuffer: LogBuffer) {
    super();
    this.processes = new Map();
    this.logBuffer = logBuffer;
    this.buffers = new Map();
    this.restartTimeouts = new Map();
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

    proc.on('error', (err: Error) => {
      this.logBuffer.addLog(id, `Process error: ${err.message}`, 'stderr');
      const current = this.processes.get(id);
      if (current && current.process === proc) {
        current.status = 'error';
        current.pid = proc.pid;
        this.emit('status-change', { processId: id, status: 'error' } as StatusChangeEvent);
      }
    });

    proc.on('exit', (code: number | null) => {
      const procInfo = this.processes.get(id);
      if (procInfo && procInfo.process === proc) {
        procInfo.status = code === 0 ? 'stopped' : 'error';
        procInfo.pid = proc.pid;
        this.emit('status-change', { processId: id, status: code === 0 ? 'stopped' : 'error' } as StatusChangeEvent);

        // Check restart policy and schedule restart if needed (Requirements 1.4, 1.5, 1.6)
        const restartConfig = procInfo.restart;
        if (restartConfig) {
          const { policy, delay } = restartConfig;
          
          if (policy === 'on-exit') {
            // Requirement 1.6: Restart whenever process exits, regardless of exit code
            this.scheduleRestart(id, delay);
          } else if (policy === 'on-error' && code !== 0) {
            // Requirement 1.5: Restart only when exit code is non-zero
            this.scheduleRestart(id, delay);
          }
          // Requirement 1.4: If policy is 'never', do nothing (existing behavior)
        }
      }
    });

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

  async restart(processId: number): Promise<void> {
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

    if (procInfo.process && !procInfo.process.killed) {
      await this.killOne(procInfo);
    }

    this.startCommand({
      id: procInfo.id,
      name: procInfo.name,
      command: procInfo.command
    });
  }

  private scheduleRestart(processId: number, delay: number): void {
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
    const restartMessage = `Restarting ${processName} in ${delaySeconds}s...`;
    this.logBuffer.addLog(processId, restartMessage, 'stdout');
    this.emit('log', { processId, line: restartMessage, source: 'stdout' } as LogEvent);

    // Schedule restart via setTimeout (Requirement 1.7)
    const timeout = setTimeout(() => {
      this.restartTimeouts.delete(processId);
      this.restart(processId);
    }, delay);

    // Store the timeout in the map
    this.restartTimeouts.set(processId, timeout);
  }
}
