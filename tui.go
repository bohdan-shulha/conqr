package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	allProcessesIndex = -1
	sidebarWidth      = 30
	tickInterval      = 100 * time.Millisecond
)

type paneFocus int

const (
	focusSidebar paneFocus = iota
	focusMain
)

type tickMsg time.Time
type restartCompleteMsg struct{}

type TUI struct {
	commands         []CommandInfo
	processManager   *ProcessManager
	logBuffer        *LogBuffer
	viewport         viewport.Model
	selectedIndex    int
	focusedPane      paneFocus
	rawMode          bool
	exiting          bool
	newLogsBelow     bool
	lastLogLineCount int
	width            int
	height           int
}

var (
	headerFocusedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ffffff")).
				Background(lipgloss.Color("#0055ff")).
				Bold(true)
	headerBlurredStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ffffff")).
				Background(lipgloss.Color("#585858"))
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#585858"))
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#eeeeee")).
			Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))
	separator   = lipgloss.NewStyle().Foreground(lipgloss.Color("#585858")).Render("│")
	upStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffaa00"))
	downStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))
	systemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#2a2a2a")).
			Italic(true)
	systemErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#ff0000")).
				Background(lipgloss.Color("#2a2a2a")).
				Italic(true)
	stderrStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))
	moreLogsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#ffaa00")).
			Bold(true)
)

func RenderTUI(commands []CommandInfo, processManager *ProcessManager, logBuffer *LogBuffer) {
	tui := TUI{
		commands:       commands,
		processManager: processManager,
		logBuffer:      logBuffer,
		viewport: viewport.New(
			viewport.WithWidth(1),
			viewport.WithHeight(1),
		),
		focusedPane: focusSidebar,
	}
	tui.viewport.MouseWheelEnabled = true
	tui.viewport.MouseWheelDelta = 3
	tui.viewport.SoftWrap = false

	processManager.StartAll(commands)
	defer processManager.KillAllForExit(100 * time.Millisecond)

	program := tea.NewProgram(tui)
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func (t TUI) Init() tea.Cmd {
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (t TUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
		t.refreshViewport(true)
	case tickMsg:
		t.refreshViewport(false)
		return t, tick()
	case restartCompleteMsg:
		return t, nil
	case tea.KeyPressMsg:
		return t.handleKey(msg)
	case tea.MouseWheelMsg:
		t.handleMouseWheel(msg)
	}

	return t, nil
}

func (t TUI) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if t.exiting {
			return t, nil
		}
		t.exiting = true
		t.noteShutdown()
		t.processManager.KillAllForExit(250 * time.Millisecond)
		return t, tea.Quit
	case "l":
		t.rawMode = !t.rawMode
		t.newLogsBelow = false
		t.refreshViewport(true)
	case "left":
		if !t.rawMode && t.focusedPane == focusMain {
			t.focusedPane = focusSidebar
		}
	case "right":
		if !t.rawMode && t.focusedPane == focusSidebar {
			t.focusedPane = focusMain
		}
	case "up":
		if !t.rawMode && t.focusedPane == focusSidebar {
			t.selectPrevious()
		} else {
			t.viewport.ScrollUp(1)
			t.syncNewLogIndicator()
		}
	case "down":
		if !t.rawMode && t.focusedPane == focusSidebar {
			t.selectNext()
		} else {
			t.viewport.ScrollDown(1)
			t.syncNewLogIndicator()
		}
	case "pgup":
		if t.rawMode || t.focusedPane == focusMain {
			t.viewport.PageUp()
			t.syncNewLogIndicator()
		}
	case "pgdown":
		if t.rawMode || t.focusedPane == focusMain {
			t.viewport.PageDown()
			t.syncNewLogIndicator()
		}
	case "home":
		if t.rawMode || t.focusedPane == focusMain {
			t.viewport.GotoTop()
			t.syncNewLogIndicator()
		}
	case "end":
		if t.rawMode || t.focusedPane == focusMain {
			t.viewport.GotoBottom()
			t.syncNewLogIndicator()
		}
	case "r":
		if !t.rawMode && t.focusedPane == focusSidebar && t.selectedIndex != allProcessesIndex {
			return t, restartCmd(t.processManager, t.commands[t.selectedIndex].ID)
		}
	}
	return t, nil
}

func (t *TUI) handleMouseWheel(msg tea.MouseWheelMsg) {
	mouse := msg.Mouse()
	if !t.rawMode && mouse.X < t.sidebarWidth()+1 {
		return
	}
	switch mouse.Button {
	case tea.MouseWheelUp:
		t.viewport.ScrollUp(3)
	case tea.MouseWheelDown:
		t.viewport.ScrollDown(3)
	}
	t.syncNewLogIndicator()
}

func (t *TUI) noteShutdown() {
	message := "› Shutting down all processes, SIGTERM signal sent"
	for _, command := range t.commands {
		t.logBuffer.Add(command.ID, message, SourceStdout, true)
	}
}

func restartCmd(processManager *ProcessManager, processID int) tea.Cmd {
	return func() tea.Msg {
		processManager.Restart(processID, true)
		return restartCompleteMsg{}
	}
}

func (t *TUI) selectPrevious() {
	if t.selectedIndex == allProcessesIndex {
		t.selectedIndex = len(t.commands) - 1
	} else if t.selectedIndex == 0 {
		t.selectedIndex = allProcessesIndex
	} else {
		t.selectedIndex--
	}
	t.refreshViewport(true)
}

func (t *TUI) selectNext() {
	if t.selectedIndex == allProcessesIndex {
		t.selectedIndex = 0
	} else if t.selectedIndex == len(t.commands)-1 {
		t.selectedIndex = allProcessesIndex
	} else {
		t.selectedIndex++
	}
	t.refreshViewport(true)
}

func (t *TUI) refreshViewport(forceBottom bool) {
	lines := t.logLines()
	wasAtBottom := t.viewport.AtBottom()
	if forceBottom || wasAtBottom {
		t.newLogsBelow = false
	} else if len(lines) > t.lastLogLineCount {
		t.newLogsBelow = true
	}

	width, height := t.logAreaSize()
	t.viewport.SetWidth(width)
	t.viewport.SetHeight(height)
	t.viewport.FillHeight = true

	t.viewport.SetContent(strings.Join(lines, "\n"))
	if forceBottom || wasAtBottom {
		t.viewport.GotoBottom()
	}
	t.lastLogLineCount = len(lines)
}

func (t *TUI) syncNewLogIndicator() {
	if t.viewport.AtBottom() {
		t.newLogsBelow = false
	}
}

func (t TUI) View() tea.View {
	var content string
	if t.rawMode {
		content = t.viewport.View()
		if t.newLogsBelow {
			content = lipgloss.JoinVertical(lipgloss.Left, content, moreLogsLine(t.width))
		}
	} else {
		content = t.fullView()
	}

	view := tea.NewView(content)
	view.AltScreen = true
	if t.rawMode {
		view.MouseMode = tea.MouseModeNone
	} else {
		view.MouseMode = tea.MouseModeCellMotion
	}
	view.WindowTitle = "conqr"
	return view
}

func (t TUI) fullView() string {
	if t.width <= 0 || t.height <= 0 {
		return ""
	}

	contentHeight := max(1, t.height-1)
	sidebar := strings.Join(t.sidebarLines(t.sidebarWidth(), contentHeight), "\n")
	main := t.mainView(contentHeight)
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, separatorColumn(contentHeight), main)

	helpText := "←→: switch | r: restart | l: logs | q: quit"
	if t.focusedPane == focusMain {
		helpText = "←→: switch | ↑↓: scroll | PageUp/Down: 10 lines | Home/End: top/bottom | l: logs | q: quit"
	}
	help := helpStyle.Width(t.width).Render(truncateDisplay(helpText, t.width))
	return lipgloss.JoinVertical(lipgloss.Left, body, help)
}

func (t TUI) mainView(height int) string {
	width, _ := t.logAreaSize()
	headerStyle := headerBlurredStyle
	prefix := " "
	if t.focusedPane == focusMain {
		headerStyle = headerFocusedStyle
		prefix = "▶"
	}

	title := " All Logs "
	if t.selectedIndex != allProcessesIndex && t.selectedIndex >= 0 && t.selectedIndex < len(t.commands) {
		command := t.commands[t.selectedIndex]
		title = " " + command.Name + " - " + command.Command + " "
	}

	restarts, crashes := t.visibleCounts()
	indicator := ""
	if restarts > 0 || crashes > 0 {
		indicator = fmt.Sprintf("↻ %d × %d ", restarts, crashes)
	}
	titleWidth := max(0, width-lipgloss.Width(indicator))
	header := headerStyle.Width(titleWidth).Render(truncateDisplay(prefix+title, titleWidth))
	if indicator != "" {
		header += headerStyle.Width(lipgloss.Width(indicator)).Render(indicator)
	}

	lines := []string{header, t.viewport.View()}
	if t.newLogsBelow {
		lines = append(lines, moreLogsLine(width))
	}
	main := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return lipgloss.NewStyle().Width(width).Height(height).Render(main)
}

func (t TUI) sidebarLines(width, height int) []string {
	lines := make([]string, 0, height)
	headerStyle := headerBlurredStyle
	headerPrefix := "  "
	if t.focusedPane == focusSidebar {
		headerStyle = headerFocusedStyle
		headerPrefix = "▶ "
	}
	lines = append(lines, headerStyle.Width(width).Render(headerPrefix+"Commands"))

	allLine := statusRow("All processes", "---", width, t.selectedIndex == allProcessesIndex, "")
	lines = append(lines, allLine)

	available := max(0, height-2)
	start := 0
	if t.selectedIndex != allProcessesIndex && t.selectedIndex >= available {
		start = t.selectedIndex - available + 1
	}
	end := min(len(t.commands), start+available)
	for index := start; index < end; index++ {
		command := t.commands[index]
		status := t.processManager.GetStatus(command.ID)
		state := t.processManager.RestartState(command.ID)
		statusText := "DOWN"
		statusColor := "#ff0000"
		if status == StatusRunning {
			statusText = "UP"
			statusColor = "#00ff00"
		} else if status == StatusError {
			statusText = "ERROR"
			statusColor = "#ffaa00"
		}
		if state.IsRestarting && status != StatusRunning {
			statusText = "› DOWN"
		}
		lines = append(lines, statusRow(command.Name, statusText, width, index == t.selectedIndex, statusColor))
	}

	blank := lipgloss.NewStyle().Width(width).Render("")
	for len(lines) < height {
		lines = append(lines, blank)
	}
	return lines[:height]
}

func statusRow(name, status string, width int, selected bool, statusColor string) string {
	statusWidth := 8
	nameWidth := max(0, width-2-statusWidth)
	prefix := "  "
	if selected {
		prefix = "• "
	}
	nameText := truncateDisplay(name, nameWidth)
	statusText := padStartDisplay(status, statusWidth)

	if selected {
		statusStyle := selectedStyle
		if statusColor != "" {
			statusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(statusColor)).
				Background(lipgloss.Color("#eeeeee"))
		}
		return selectedStyle.Width(2+nameWidth).Render(prefix+nameText) +
			statusStyle.Width(statusWidth).Render(statusText)
	}

	base := dimStyle.Width(2 + nameWidth).Render(prefix + nameText)
	if statusColor == "#00ff00" {
		return base + upStyle.Width(statusWidth).Render(statusText)
	}
	if statusColor == "#ffaa00" {
		return base + errorStyle.Width(statusWidth).Render(statusText)
	}
	if statusColor == "#ff0000" {
		return base + downStyle.Width(statusWidth).Render(statusText)
	}
	return base + lipgloss.NewStyle().Foreground(lipgloss.Color("#d7d7d7")).Width(statusWidth).Render(statusText)
}

func (t TUI) logLines() []string {
	logs := t.currentLogs()
	lines := make([]string, 0, len(logs))
	width, _ := t.logAreaSize()
	for _, entry := range logs {
		lines = append(lines, renderLogLine(entry, width, t.selectedIndex == allProcessesIndex, t.commands, !t.rawMode))
	}
	return lines
}

func renderLogLine(entry LogEntry, width int, unified bool, commands []CommandInfo, useColors bool) string {
	line := stripANSI(entry.Line)
	if unified && entry.HasID {
		line = "[" + processName(entry.ProcessID, commands) + "] " + line
	}

	if !useColors {
		return line
	}
	if entry.IsSystem && strings.HasPrefix(entry.Line, "× Process exited with code") {
		return systemErrorStyle.Width(width).Render(truncateDisplay("  "+line, width))
	}
	if entry.IsSystem {
		return systemStyle.Width(width).Render(truncateDisplay("  "+line, width))
	}
	if color := detectANSIColor(entry.Line); color != "" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ansiColorHex(color))).Render(line)
	}
	if entry.Source == SourceStderr {
		return stderrStyle.Render(line)
	}
	return line
}

func processName(processID int, commands []CommandInfo) string {
	for _, command := range commands {
		if command.ID == processID {
			return command.Name
		}
	}
	return fmt.Sprintf("%d", processID)
}

func (t TUI) visibleCounts() (int, int) {
	restarts := 0
	crashes := 0
	if t.selectedIndex == allProcessesIndex {
		for _, command := range t.commands {
			state := t.processManager.RestartState(command.ID)
			restarts += state.RestartCount
			crashes += state.CrashCount
		}
		return restarts, crashes
	}
	if t.selectedIndex >= 0 && t.selectedIndex < len(t.commands) {
		state := t.processManager.RestartState(t.commands[t.selectedIndex].ID)
		return state.RestartCount, state.CrashCount
	}
	return 0, 0
}

func (t TUI) currentLogs() []LogEntry {
	if t.selectedIndex == allProcessesIndex {
		return t.logBuffer.UnifiedLogs()
	}
	if t.selectedIndex >= 0 && t.selectedIndex < len(t.commands) {
		return t.logBuffer.Logs(t.commands[t.selectedIndex].ID)
	}
	return nil
}

func (t TUI) sidebarWidth() int {
	if t.width <= 0 {
		return sidebarWidth
	}
	return min(sidebarWidth, max(20, t.width/3))
}

func (t TUI) logAreaSize() (int, int) {
	indicatorHeight := 0
	if t.newLogsBelow {
		indicatorHeight = 1
	}
	if t.rawMode {
		return max(1, t.width), max(1, t.height-indicatorHeight)
	}
	return max(1, t.width-t.sidebarWidth()-1), max(1, t.height-2-indicatorHeight)
}

func separatorColumn(height int) string {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = separator
	}
	return strings.Join(lines, "\n")
}

func moreLogsLine(width int) string {
	return moreLogsStyle.Width(width).Render(truncateDisplay(" ▼ More logs below - Press END to jump to bottom ", width))
}

func truncateDisplay(text string, width int) string {
	if width <= 0 {
		return ""
	}
	for lipgloss.Width(text) > width {
		runes := []rune(text)
		if len(runes) == 0 {
			return ""
		}
		text = string(runes[:len(runes)-1])
	}
	return text
}

func padStartDisplay(text string, width int) string {
	if lipgloss.Width(text) >= width {
		return truncateDisplay(text, width)
	}
	return strings.Repeat(" ", width-lipgloss.Width(text)) + text
}

func ansiColorHex(code string) string {
	switch code {
	case "31":
		return "#ff0000"
	case "33":
		return "#ffaa00"
	case "32":
		return "#00ff00"
	case "34":
		return "#5555ff"
	case "35":
		return "#ff55ff"
	case "36":
		return "#55ffff"
	case "37":
		return "#ffffff"
	case "30":
		return "#000000"
	default:
		return "#ffffff"
	}
}
