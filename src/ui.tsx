import { useState, useEffect } from 'react';
import { render, Box, Text, useInput, useApp } from 'ink';
import { CommandInfo } from './cli.js';
import { ProcessManager } from './process-manager.js';
import { LogBuffer, LogEntry } from './log-buffer.js';

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

  useEffect(() => {
    const updateStatuses = () => {
      const newStatuses = new Map<number, string>();
      commands.forEach(cmd => {
        newStatuses.set(cmd.id, processManager.getStatus(cmd.id));
      });
      setStatuses(newStatuses);
    };

    const updateLogs = () => {
      const unifiedView = selectedIndex === ALL_PROCESSES_INDEX;
      if (unifiedView) {
        setLogs([...logBuffer.getUnifiedLogs()]);
      } else {
        const selectedCmd = commands[selectedIndex];
        setLogs([...logBuffer.getLogs(selectedCmd.id)]);
      }
    };

    processManager.on('status-change', updateStatuses);
    processManager.on('log', updateLogs);

    updateStatuses();
    updateLogs();

    const interval = setInterval(() => {
      updateLogs();
      updateStatuses();
    }, 100);

    return () => {
      clearInterval(interval);
      processManager.removeAllListeners('status-change');
      processManager.removeAllListeners('log');
    };
  }, [commands, processManager, logBuffer, selectedIndex]);

  useEffect(() => {
    setLogScrollOffset(Infinity);
  }, [selectedIndex]);

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
  const displayHeight = (process.stdout.rows || 24) - 1;
  let startIndex: number;

  const wasAtBottom = logScrollOffset === Infinity ||
    (logScrollOffset >= logs.length - displayHeight && logs.length > displayHeight);

  if (wasAtBottom) {
    startIndex = Math.max(0, logs.length - displayHeight);
    if (logScrollOffset !== Infinity && logs.length > 0) {
      setLogScrollOffset(Infinity);
    }
  } else {
    startIndex = Math.min(logScrollOffset, Math.max(0, logs.length - displayHeight));
  }

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
        const name = cmd.name.substring(0, width - 8);
        const statusText = status === 'running' ? 'UP' : 'DOWN';
        const statusColor = status === 'running' ? '#00ff00' : '#ff0000';

        return (
          <Box key={cmd.id} width={width} height={1}>
            <Text backgroundColor={itemBg} color={itemFg} bold={isSelected}>
              {dotText}
            </Text>
            <Text backgroundColor={itemBg} color={itemFg} bold={isSelected}>
              {name.padEnd(width - 8)}
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
          let line = log.line;
          if (unifiedView && log.processId !== undefined) {
            const cmd = commands.find(c => c.id === log.processId);
            const prefix = `[${cmd ? cmd.name : log.processId}] `;
            line = prefix + line;
          }
          const lineColor = log.source === 'stderr' ? '#ff0000' : '#ffffff';
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
