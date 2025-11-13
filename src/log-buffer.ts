const MAX_LINES_PER_PROCESS = 1000;

export interface LogEntry {
  line: string;
  source: 'stdout' | 'stderr';
  timestamp: number;
  processId?: number;
}

export class LogBuffer {
  private buffers: Map<number, LogEntry[]>;
  private unifiedBuffer: LogEntry[];
  private maxLines: number;

  constructor() {
    this.buffers = new Map();
    this.unifiedBuffer = [];
    this.maxLines = MAX_LINES_PER_PROCESS;
  }

  addLog(processId: number, line: string, source: 'stdout' | 'stderr' = 'stdout'): void {
    if (!this.buffers.has(processId)) {
      this.buffers.set(processId, []);
    }

    const buffer = this.buffers.get(processId)!;
    buffer.push({ line, source, timestamp: Date.now() });

    if (buffer.length > this.maxLines) {
      buffer.shift();
    }

    this.unifiedBuffer.push({ processId, line, source, timestamp: Date.now() });
    if (this.unifiedBuffer.length > this.maxLines * 10) {
      this.unifiedBuffer.shift();
    }
  }

  getLogs(processId: number): LogEntry[] {
    return this.buffers.get(processId) || [];
  }

  getUnifiedLogs(): LogEntry[] {
    return this.unifiedBuffer;
  }

  clear(processId?: number): void {
    if (processId !== undefined) {
      this.buffers.delete(processId);
    } else {
      this.buffers.clear();
      this.unifiedBuffer = [];
    }
  }
}
