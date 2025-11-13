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

  constructor(logBuffer: LogBuffer) {
    super();
    this.processes = new Map();
    this.logBuffer = logBuffer;
    this.buffers = new Map();
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
    ];
    return hasRedAnsi || errorPatterns.some(pattern => pattern.test(line));
  }

  startCommand(commandInfo: CommandInfo): ChildProcess {
    const { id, name, command } = commandInfo;

    const proc = spawn(command, {
      shell: true,
      stdio: ['ignore', 'pipe', 'pipe']
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
      this.processes.set(id, { ...commandInfo, status: 'error', process: proc, pid: proc.pid });
      this.emit('status-change', { processId: id, status: 'error' } as StatusChangeEvent);
    });

    proc.on('exit', (code: number | null) => {
      const procInfo = this.processes.get(id);
      if (procInfo) {
        procInfo.status = code === 0 ? 'stopped' : 'error';
        procInfo.pid = proc.pid;
      }
      this.emit('status-change', { processId: id, status: code === 0 ? 'stopped' : 'error' } as StatusChangeEvent);
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
    } else if (!hasRecentError && procInfo.status === 'error' && !procInfo.process.killed) {
      procInfo.status = 'running';
      this.emit('status-change', { processId, status: 'running' } as StatusChangeEvent);
    }
  }

  private killProcess(procInfo: ProcessInfo, signal: NodeJS.Signals): void {
    if (!procInfo.process || procInfo.process.killed) {
      return;
    }

    try {
      procInfo.process.kill(signal);
    } catch {
    }
  }

  async killAll(): Promise<void> {
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
}
