package main

import (
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestTUIKeyboardNavigationAndScroll(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	commands := []CommandInfo{
		{ID: 0, Name: "one", Command: "echo one"},
		{ID: 1, Name: "two", Command: "echo two"},
	}
	for i := 0; i < 30; i++ {
		logBuffer.Add(0, "line", SourceStdout, false)
	}

	tui := TUI{
		commands:       commands,
		processManager: manager,
		logBuffer:      logBuffer,
		viewport:       viewport.New(viewport.WithWidth(20), viewport.WithHeight(5)),
		width:          80,
		height:         24,
		focusedPane:    focusSidebar,
	}
	tui.refreshViewport(true)

	model, _ := tui.handleKey(keyPress("down"))
	tui = model.(TUI)
	if tui.selectedIndex != 1 {
		t.Fatalf("expected selected index 1, got %d", tui.selectedIndex)
	}

	model, _ = tui.handleKey(keyPress("down"))
	tui = model.(TUI)
	if tui.selectedIndex != allProcessesIndex {
		t.Fatalf("expected all processes selection, got %d", tui.selectedIndex)
	}

	model, _ = tui.handleKey(keyPress("right"))
	tui = model.(TUI)
	if tui.focusedPane != focusMain {
		t.Fatalf("expected main pane focus, got %v", tui.focusedPane)
	}

	before := tui.viewport.YOffset()
	model, _ = tui.handleKey(keyPress("up"))
	tui = model.(TUI)
	if after := tui.viewport.YOffset(); after >= before {
		t.Fatalf("expected viewport to scroll up from %d, got %d", before, after)
	}
}

func TestTUIRestartRunsAsCommand(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	commands := []CommandInfo{{ID: 0, Name: "one", Command: "sleep 1"}}
	tui := TUI{
		commands:       commands,
		processManager: manager,
		logBuffer:      logBuffer,
		viewport:       viewport.New(viewport.WithWidth(20), viewport.WithHeight(5)),
		focusedPane:    focusSidebar,
	}

	model, cmd := tui.handleKey(keyPress("r"))
	tui = model.(TUI)
	if cmd == nil {
		t.Fatal("expected restart to run as Bubble Tea command")
	}
	if tui.selectedIndex != 0 {
		t.Fatalf("restart should not change selection, got %d", tui.selectedIndex)
	}
}

func TestWaitForProcessExitReturnsPromptly(t *testing.T) {
	command := shellCommand("exit 0")
	if err := command.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_ = command.Wait()
		close(done)
	}()
	<-done

	start := time.Now()
	waitForProcessExit(command, time.Second)
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("expected prompt wait return for exited process, took %s", elapsed)
	}
}

func TestTUINewLogsBelowIndicator(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	commands := []CommandInfo{{ID: 0, Name: "one", Command: "echo one"}}
	for i := 0; i < 30; i++ {
		logBuffer.Add(0, "line", SourceStdout, false)
	}

	tui := TUI{
		commands:       commands,
		processManager: manager,
		logBuffer:      logBuffer,
		viewport:       viewport.New(viewport.WithWidth(20), viewport.WithHeight(5)),
		width:          80,
		height:         24,
		focusedPane:    focusMain,
	}
	tui.refreshViewport(true)

	model, _ := tui.handleKey(keyPress("home"))
	tui = model.(TUI)
	offset := tui.viewport.YOffset()
	visible := tui.viewport.View()
	logBuffer.Add(0, "new line", SourceStdout, false)
	tui.refreshViewport(false)

	if !tui.newLogsBelow {
		t.Fatal("expected new logs below indicator when logs arrive while scrolled up")
	}
	if got := tui.viewport.YOffset(); got != offset {
		t.Fatalf("expected viewport offset to stay at %d, got %d", offset, got)
	}
	if got := tui.viewport.View(); got != visible {
		t.Fatal("expected visible log viewport to stay unchanged when logs append below")
	}
	if !strings.Contains(tui.mainView(23), "More logs below") {
		t.Fatal("expected main view to render more logs indicator")
	}

	model, _ = tui.handleKey(keyPress("end"))
	tui = model.(TUI)
	if tui.newLogsBelow {
		t.Fatal("expected new logs indicator to clear at bottom")
	}
}

func TestTUIViewDisablesMouseInRawMode(t *testing.T) {
	tui := TUI{rawMode: true}
	if view := tui.View(); view.MouseMode != tea.MouseModeNone {
		t.Fatalf("expected raw mode to disable mouse reporting, got %v", view.MouseMode)
	}

	tui.rawMode = false
	if view := tui.View(); view.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("expected normal mode to enable mouse reporting, got %v", view.MouseMode)
	}
}

func TestSystemLogLineRendersFullWidth(t *testing.T) {
	line := renderLogLine(LogEntry{
		Line:     "› Service starting",
		Source:   SourceStdout,
		IsSystem: true,
	}, 40, false, nil, true)

	if width := lipgloss.Width(line); width != 40 {
		t.Fatalf("expected full-width system log line, got width %d", width)
	}
}

func keyPress(value string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Text: value, Code: []rune(value)[0]})
}
