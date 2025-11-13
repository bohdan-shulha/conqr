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

  private killProcess(procInfo: ProcessInfo, signal: NodeJS.Signals): void {
    if (!procInfo.process || procInfo.process.killed) {
      return;
    }

    try {
      procInfo.process.kill(signal);
    } catch {
    }
  }

  killAll(): void {
    this.processes.forEach((procInfo) => {
      this.killProcess(procInfo, 'SIGKILL');
    });

    this.processes.forEach((procInfo) => {
      if (procInfo.process && !procInfo.process.killed) {
        this.killProcess(procInfo, 'SIGTERM');
      }
    });
  }
}
