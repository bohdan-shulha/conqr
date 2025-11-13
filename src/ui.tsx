import { useState, useEffect, useRef } from 'react';
import { render, Box, Text, useInput, useApp } from 'ink';
import stripAnsi from 'strip-ansi';
import { CommandInfo } from './cli.js';
import { ProcessManager } from './process-manager.js';
import { LogBuffer, LogEntry } from './log-buffer.js';

function detectAnsiColor(line: string): string | null {
  const ansiColorRegex = /\x1b\[(\d+)(?:;(\d+))?(?:;(\d+))?(?:;(\d+))?(?:;(\d+))?m/g;
  let match;
  let lastColor = null;

  while ((match = ansiColorRegex.exec(line)) !== null) {
    const code = match[1];
    const code2 = match[2];
    const code3 = match[3];
    const code4 = match[4];
    const code5 = match[5];

    if (code === '0') {
      lastColor = null;
    } else if (code === '31' || code === '91') {
      lastColor = '#ff0000';
    } else if (code === '33' || code === '93') {
      lastColor = '#ffaa00';
    } else if (code === '32' || code === '92') {
      lastColor = '#00ff00';
    } else if (code === '34' || code === '94') {
      lastColor = '#5555ff';
    } else if (code === '35' || code === '95') {
      lastColor = '#ff55ff';
    } else if (code === '36' || code === '96') {
      lastColor = '#55ffff';
    } else if (code === '37' || code === '97') {
      lastColor = '#ffffff';
    } else if (code === '30' || code === '90') {
      lastColor = '#000000';
    } else if (code === '38' && code2 === '5' && code3) {
      const color256 = parseInt(code3);
      if (color256 >= 0 && color256 <= 15) {
        const basicColors = [
          '#000000', '#800000', '#008000', '#808000',
          '#000080', '#800080', '#008080', '#c0c0c0',
          '#808080', '#ff0000', '#00ff00', '#ffff00',
          '#0000ff', '#ff00ff', '#00ffff', '#ffffff'
        ];
        lastColor = basicColors[color256] || null;
      } else if (color256 >= 16 && color256 <= 231) {
        const r = Math.floor((color256 - 16) / 36);
        const g = Math.floor(((color256 - 16) % 36) / 6);
        const b = (color256 - 16) % 6;
        const rVal = r === 0 ? 0 : 55 + r * 40;
        const gVal = g === 0 ? 0 : 55 + g * 40;
        const bVal = b === 0 ? 0 : 55 + b * 40;
        lastColor = `#${rVal.toString(16).padStart(2, '0')}${gVal.toString(16).padStart(2, '0')}${bVal.toString(16).padStart(2, '0')}`;
      } else if (color256 >= 232 && color256 <= 255) {
        const gray = 8 + (color256 - 232) * 10;
        const grayHex = gray.toString(16).padStart(2, '0');
        lastColor = `#${grayHex}${grayHex}${grayHex}`;
      }
    } else if (code === '38' && code2 === '2' && code3 && code4 && code5) {
      const r = parseInt(code3);
      const g = parseInt(code4);
      const b = parseInt(code5);
      lastColor = `#${r.toString(16).padStart(2, '0')}${g.toString(16).padStart(2, '0')}${b.toString(16).padStart(2, '0')}`;
    }
  }

  return lastColor;
}

type PaneFocus = 'sidebar' | 'main';

interface TUIProps {
  commands: CommandInfo[];
  processManager: ProcessManager;
  logBuffer: LogBuffer;
}

const ALL_PROCESSES_INDEX = -1;

export function TUI({ commands, processManager, logBuffer }: TUIProps) {
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [focusedPane, setFocusedPane] = useState<PaneFocus>('sidebar');
  const [logScrollOffset, setLogScrollOffset] = useState(0);
  const [statuses, setStatuses] = useState<Map<number, string>>(new Map());
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const { exit } = useApp();

  const sidebarWidth = 30;

  const prevLogsRef = useRef<LogEntry[]>([]);
  const prevStatusesRef = useRef<Map<number, string>>(new Map());

  useEffect(() => {
    const updateStatuses = () => {
      const newStatuses = new Map<number, string>();
      commands.forEach(cmd => {
        newStatuses.set(cmd.id, processManager.getStatus(cmd.id));
      });

      const prevStatuses = prevStatusesRef.current;
      let hasChanged = false;
      if (prevStatuses.size !== newStatuses.size) {
        hasChanged = true;
      } else {
        for (const [id, status] of newStatuses) {
          if (prevStatuses.get(id) !== status) {
            hasChanged = true;
            break;
          }
        }
      }

      if (hasChanged) {
        prevStatusesRef.current = newStatuses;
        setStatuses(newStatuses);
      }
    };

    const updateLogs = () => {
      const unifiedView = selectedIndex === ALL_PROCESSES_INDEX;
      const bufferLogs = unifiedView
        ? logBuffer.getUnifiedLogs()
        : (() => {
            const selectedCmd = commands[selectedIndex];
            return logBuffer.getLogs(selectedCmd.id);
          })();

      const prevLogs = prevLogsRef.current;
      const prevLength = prevLogs.length;
      const newLength = bufferLogs.length;

      if (prevLength !== newLength || (newLength > 0 && prevLogs[prevLength - 1]?.timestamp !== bufferLogs[newLength - 1]?.timestamp)) {
        const newLogs = [...bufferLogs];
        prevLogsRef.current = newLogs;
        setLogs(newLogs);
      }
    };

    processManager.on('status-change', updateStatuses);
    processManager.on('log', updateLogs);

    updateStatuses();
    updateLogs();

    return () => {
      processManager.removeAllListeners('status-change');
      processManager.removeAllListeners('log');
    };
  }, [commands, processManager, logBuffer, selectedIndex]);

  useEffect(() => {
    setLogScrollOffset(Infinity);
    prevLogsRef.current = [];
  }, [selectedIndex]);

  const prevLogsLengthRef = useRef<number>(0);
  const displayHeight = (process.stdout.rows || 24) - 1;

  useEffect(() => {
    const wasAtBottom = logScrollOffset === Infinity ||
      (logScrollOffset >= logs.length - displayHeight && logs.length > displayHeight);

    if (wasAtBottom && logs.length > prevLogsLengthRef.current && logScrollOffset !== Infinity) {
      setLogScrollOffset(Infinity);
    }

    prevLogsLengthRef.current = logs.length;
  }, [logs.length, logScrollOffset, displayHeight]);

  useInput((input: string, key: any) => {
    if (key.leftArrow && focusedPane === 'main') {
      setFocusedPane('sidebar');
    } else if (key.rightArrow && focusedPane === 'sidebar') {
      setFocusedPane('main');
    } else if (key.upArrow) {
      if (focusedPane === 'sidebar') {
        if (selectedIndex === ALL_PROCESSES_INDEX) {
          setSelectedIndex(commands.length - 1);
        } else if (selectedIndex === 0) {
          setSelectedIndex(ALL_PROCESSES_INDEX);
        } else {
          setSelectedIndex(selectedIndex - 1);
        }
        setLogScrollOffset(Infinity);
      } else {
        setLogScrollOffset((prev: number) => {
          if (prev === Infinity) {
            const currentLogs = unifiedView ? logBuffer.getUnifiedLogs() : logBuffer.getLogs(commands[selectedIndex].id);
            return Math.max(0, currentLogs.length - displayHeight - 1);
          }
          return Math.max(0, prev - 1);
        });
      }
    } else if (key.downArrow) {
      if (focusedPane === 'sidebar') {
        if (selectedIndex === ALL_PROCESSES_INDEX) {
          setSelectedIndex(0);
        } else if (selectedIndex === commands.length - 1) {
          setSelectedIndex(ALL_PROCESSES_INDEX);
        } else {
          setSelectedIndex(selectedIndex + 1);
        }
        setLogScrollOffset(Infinity);
      } else {
        setLogScrollOffset((prev: number) => {
          if (prev === Infinity) {
            return Infinity;
          }
          const currentLogs = unifiedView ? logBuffer.getUnifiedLogs() : logBuffer.getLogs(commands[selectedIndex].id);
          const maxScroll = Math.max(0, currentLogs.length - displayHeight);
          return Math.min(prev + 1, maxScroll);
        });
      }
    } else if (key.pageUp && focusedPane === 'main') {
      setLogScrollOffset((prev: number) => {
        if (prev === Infinity) {
          const currentLogs = unifiedView ? logBuffer.getUnifiedLogs() : logBuffer.getLogs(commands[selectedIndex].id);
          return Math.max(0, currentLogs.length - displayHeight - 10);
        }
        return Math.max(0, prev - 10);
      });
    } else if (key.pageDown && focusedPane === 'main') {
      setLogScrollOffset((prev: number) => {
        if (prev === Infinity) {
          return Infinity;
        }
        const currentLogs = unifiedView ? logBuffer.getUnifiedLogs() : logBuffer.getLogs(commands[selectedIndex].id);
        const maxScroll = Math.max(0, currentLogs.length - displayHeight);
        return Math.min(prev + 10, maxScroll);
      });
    } else if (key.home && focusedPane === 'main') {
      setLogScrollOffset(0);
    } else if (key.end && focusedPane === 'main') {
      setLogScrollOffset(Infinity);
    } else if (input === 'q' || input === 'Q' || (key.ctrl && input === 'c')) {
      processManager.killAll();
      exit();
    }
  });

  const unifiedView = selectedIndex === ALL_PROCESSES_INDEX;

  const wasAtBottom = logScrollOffset === Infinity ||
    (logScrollOffset >= logs.length - displayHeight && logs.length > displayHeight);

  const startIndex = wasAtBottom
    ? Math.max(0, logs.length - displayHeight)
    : Math.min(logScrollOffset, Math.max(0, logs.length - displayHeight));

  const displayLogs = logs.slice(startIndex, startIndex + displayHeight);

  const terminalWidth = process.stdout.columns || 80;
  const terminalHeight = process.stdout.rows || 24;
  const contentHeight = terminalHeight - 1;

  const helpText = focusedPane === 'sidebar'
    ? '←→: switch | q: quit'
    : '←→: switch | ↑↓: scroll | PageUp/Down: 10 lines | Home/End: top/bottom | q: quit';

  return (
    <Box flexDirection="column" width={terminalWidth} height={terminalHeight}>
      <Box flexDirection="row" width={terminalWidth} height={contentHeight}>
        <Sidebar
          width={sidebarWidth}
          height={contentHeight}
          commands={commands}
          selectedIndex={selectedIndex}
          statuses={statuses}
          focusedPane={focusedPane}
        />
        <Separator height={contentHeight} />
      <MainPane
        width={terminalWidth - sidebarWidth - 1}
        height={contentHeight}
        unifiedView={unifiedView}
        selectedCommand={unifiedView ? null : commands[selectedIndex]}
        logs={displayLogs}
        focusedPane={focusedPane}
        commands={commands}
        logScrollOffset={logScrollOffset}
        totalLogs={logs.length}
        displayHeight={displayHeight}
      />
      </Box>
      <Box width={terminalWidth} height={1}>
        <Text backgroundColor="#585858" color="#ffffff">
          {helpText.padEnd(terminalWidth)}
        </Text>
      </Box>
    </Box>
  );
}

interface SidebarProps {
  width: number;
  height: number;
  commands: CommandInfo[];
  selectedIndex: number;
  statuses: Map<number, string>;
  focusedPane: PaneFocus;
}

function Sidebar({ width, height, commands, selectedIndex, statuses, focusedPane }: SidebarProps) {
  const headerBg = focusedPane === 'sidebar' ? '#0055ff' : '#585858';
  const headerFg = '#ffffff';
  const headerText = focusedPane === 'sidebar' ? '▶ Commands ' : '  Commands ';

  const availableHeight = height - 1;
  let renderIndex = 0;

  const isAllProcessesSelected = selectedIndex === ALL_PROCESSES_INDEX;
  const allProcessesBg = isAllProcessesSelected ? '#eeeeee' : undefined;
  const allProcessesFg = isAllProcessesSelected ? '#000000' : '#d7d7d7';
  const allProcessesDot = isAllProcessesSelected ? '• ' : '  ';
  const allProcessesName = 'All processes'.substring(0, width - 8);
  const allProcessesNamePadded = allProcessesName.padEnd(width - 8);

  const maxCommandsToShow = availableHeight - renderIndex;
  let startCmdIndex = 0;

  if (selectedIndex !== ALL_PROCESSES_INDEX && selectedIndex >= maxCommandsToShow) {
    startCmdIndex = selectedIndex - maxCommandsToShow + 1;
  }

  const visibleCommands = commands.slice(startCmdIndex, startCmdIndex + maxCommandsToShow);

  const helpText = focusedPane === 'sidebar'
    ? '←→: switch | q: quit'
    : '←→: switch | ↑↓: scroll | Home/End: top/bottom | q: quit';

  return (
    <Box flexDirection="column" width={width} height={height}>
      <Box width={width} height={1}>
        <Text backgroundColor={headerBg} color={headerFg} bold={focusedPane === 'sidebar'}>
          {headerText.padEnd(width)}
        </Text>
      </Box>
      <Box width={width} height={1}>
        <Text backgroundColor={allProcessesBg} color={allProcessesFg} bold={isAllProcessesSelected}>
          {allProcessesDot}
        </Text>
        <Text backgroundColor={allProcessesBg} color={allProcessesFg} bold={isAllProcessesSelected}>
          {allProcessesNamePadded}
        </Text>
        <Text backgroundColor={allProcessesBg} color="#d7d7d7">
          {' '}
        </Text>
        <Text backgroundColor={allProcessesBg} color="#d7d7d7">---</Text>
      </Box>
      {visibleCommands.map((cmd, idx) => {
        const actualIndex = startCmdIndex + idx;
        const status = statuses.get(cmd.id) || 'unknown';
        const isSelected = actualIndex === selectedIndex;
        const itemBg = isSelected ? '#eeeeee' : undefined;
        const itemFg = isSelected ? '#000000' : '#d7d7d7';
        const dotText = isSelected ? '• ' : '  ';
        const statusText = status === 'running' ? 'UP' : status === 'error' ? 'ERROR' : 'DOWN';
        const statusColor = status === 'running' ? '#00ff00' : status === 'error' ? '#ffaa00' : '#ff0000';
        const statusWidth = statusText.length + 1;
        const nameMaxWidth = width - 2 - statusWidth;
        const name = cmd.name.substring(0, nameMaxWidth);

        return (
          <Box key={cmd.id} width={width} height={1}>
            <Text backgroundColor={itemBg} color={itemFg} bold={isSelected}>
              {dotText}
            </Text>
            <Text backgroundColor={itemBg} color={itemFg} bold={isSelected}>
              {name.padEnd(nameMaxWidth)}
            </Text>
            <Text backgroundColor={itemBg} color={statusColor}>
              {' ' + statusText}
            </Text>
          </Box>
        );
      })}
    </Box>
  );
}

function Separator({ height }: { height: number }) {
  return (
    <Box flexDirection="column" width={1} height={height}>
      {Array.from({ length: height }, (_, i) => (
        <Text key={i} color="#585858">│</Text>
      ))}
    </Box>
  );
}

interface MainPaneProps {
  width: number;
  height: number;
  unifiedView: boolean;
  selectedCommand: CommandInfo | null;
  logs: LogEntry[];
  focusedPane: PaneFocus;
  commands: CommandInfo[];
  logScrollOffset: number;
  totalLogs: number;
  displayHeight: number;
}

function MainPane({ width, height, unifiedView, selectedCommand, logs, focusedPane, commands, logScrollOffset, totalLogs, displayHeight }: MainPaneProps) {
  const title = unifiedView ? ' All Logs ' : ` ${selectedCommand?.name || ''} - ${selectedCommand?.command || ''} `;
  const headerBg = focusedPane === 'main' ? '#0055ff' : '#585858';
  const headerFg = '#ffffff';
  const headerText = focusedPane === 'main' ? '▶' + title : ' ' + title;

  const isScrolledUp = logScrollOffset !== Infinity;
  const hasMoreBelow = isScrolledUp && totalLogs > displayHeight && logScrollOffset < totalLogs - displayHeight;
  const logAreaHeight = height - 1 - (hasMoreBelow ? 1 : 0);

  return (
    <Box flexDirection="column" width={width} height={height}>
      <Box width={width} height={1}>
        <Text backgroundColor={headerBg} color={headerFg} bold={focusedPane === 'main'}>
          {headerText.padEnd(width)}
        </Text>
      </Box>
      <Box flexDirection="column" width={width} height={logAreaHeight}>
        {logs.slice(0, logAreaHeight).map((log, i) => {
          const ansiColor = detectAnsiColor(log.line);
          let line = stripAnsi(log.line);
          if (unifiedView && log.processId !== undefined) {
            const cmd = commands.find(c => c.id === log.processId);
            const prefix = `[${cmd ? cmd.name : log.processId}] `;
            line = prefix + line;
          }
          const lineColor = ansiColor || (log.source === 'stderr' ? '#ff0000' : '#ffffff');
          const truncated = line.substring(0, width);

          return (
            <Text key={i} color={lineColor}>
              {truncated.padEnd(width)}
            </Text>
          );
        })}
      </Box>
      {hasMoreBelow && (
        <Box width={width} height={1}>
          <Text backgroundColor="#ffaa00" color="#000000" bold>
            {' ▼ More logs below - Press END to jump to bottom '.padEnd(width)}
          </Text>
        </Box>
      )}
    </Box>
  );
}

export function renderTUI(commands: CommandInfo[], processManager: ProcessManager, logBuffer: LogBuffer) {
  processManager.startAll(commands);

  const cleanup = () => {
    processManager.killAll();
  };

  process.on('SIGINT', cleanup);
  process.on('SIGTERM', cleanup);

  const { waitUntilExit } = render(
    <TUI commands={commands} processManager={processManager} logBuffer={logBuffer} />
  );

  waitUntilExit().then(() => {
    cleanup();
  });
}
