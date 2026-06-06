package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testCommand(script string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("node -e %q", script)
	}
	return fmt.Sprintf("%q -e %q", os.Args[0], script)
}

func shellSleepCommand(duration time.Duration, exitCode int) string {
	seconds := duration.Seconds()
	return fmt.Sprintf("sleep %.3f; exit %d", seconds, exitCode)
}

func waitUntil(t *testing.T, predicate func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out after %s", timeout)
}

func countStartLogs(buffer *LogBuffer, processID int) int {
	count := 0
	for _, entry := range buffer.Logs(processID) {
		if entry.Line == "› Service starting" {
			count++
		}
	}
	return count
}

func TestScheduledRestartDoesNotRestartProcessAlreadyRunningAgain(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	if err := manager.StartCommand(CommandInfo{
		ID:      1,
		Name:    "steady",
		Command: "sleep 5",
	}); err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}

	waitUntil(t, func() bool {
		return manager.GetStatus(1) == StatusRunning && countStartLogs(logBuffer, 1) == 1
	}, time.Second)

	manager.mu.Lock()
	runID := manager.processes[1].RunID
	manager.mu.Unlock()
	manager.scheduleRestart(1, 0, intPtr(0), runID)

	time.Sleep(50 * time.Millisecond)

	if got := countStartLogs(logBuffer, 1); got != 1 {
		t.Fatalf("expected 1 start log, got %d", got)
	}
	if status := manager.GetStatus(1); status != StatusRunning {
		t.Fatalf("expected running status, got %s", status)
	}
	if state := manager.RestartState(1); state.IsRestarting {
		t.Fatalf("expected restarting state to clear, got %+v", state)
	}
}

func TestConcurrentManualRestartsOnlyStartOneReplacementProcess(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	if err := manager.StartCommand(CommandInfo{
		ID:      1,
		Name:    "short-lived",
		Command: shellSleepCommand(100*time.Millisecond, 0),
	}); err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}

	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusRunning }, time.Second)
	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusStopped }, 2*time.Second)

	done := make(chan struct{})
	go func() {
		defer close(done)
		go manager.Restart(1, true)
		manager.Restart(1, true)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("restarts did not complete")
	}

	waitUntil(t, func() bool { return countStartLogs(logBuffer, 1) >= 2 }, time.Second)
	time.Sleep(50 * time.Millisecond)

	if got := countStartLogs(logBuffer, 1); got != 2 {
		var lines []string
		for _, entry := range logBuffer.Logs(1) {
			lines = append(lines, entry.Line)
		}
		t.Fatalf("expected 2 start logs, got %d: %s", got, strings.Join(lines, " | "))
	}
	if state := manager.RestartState(1); state.RestartCount != 1 {
		t.Fatalf("expected 1 restart, got %+v", state)
	}
}

func TestKillAllCancelsPendingRestart(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	if err := manager.StartCommand(CommandInfo{
		ID:      1,
		Name:    "restartable",
		Command: shellSleepCommand(50*time.Millisecond, 0),
		Restart: &RestartConfig{
			Policy: RestartOnExit,
			Delay:  100,
		},
	}); err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}

	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusStopped }, time.Second)
	manager.KillAll()
	time.Sleep(200 * time.Millisecond)

	if got := countStartLogs(logBuffer, 1); got != 1 {
		t.Fatalf("expected pending restart to be canceled, got %d starts", got)
	}
}

func TestKillAllForExitReturnsQuicklyForTermIgnoringProcess(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAllForExit(50 * time.Millisecond)

	if err := manager.StartCommand(CommandInfo{
		ID:      1,
		Name:    "ignore-term",
		Command: "trap '' TERM; sleep 10",
	}); err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}

	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusRunning }, time.Second)

	start := time.Now()
	manager.KillAllForExit(100 * time.Millisecond)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("expected forced shutdown to return quickly, took %s", elapsed)
	}
}

func TestStopKillsRunningProcess(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	if err := manager.StartCommand(CommandInfo{
		ID:      1,
		Name:    "long",
		Command: "sleep 10",
	}); err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}

	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusRunning }, time.Second)

	manager.Stop(1, true)

	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusStopped }, 2*time.Second)

	if !manager.IsManuallyStopped(1) {
		t.Fatal("expected process to be manually stopped")
	}
	if got := countStartLogs(logBuffer, 1); got != 1 {
		t.Fatalf("expected 1 start log, got %d", got)
	}
}

func TestStopCancelsPendingRestart(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	if err := manager.StartCommand(CommandInfo{
		ID:      1,
		Name:    "restartable",
		Command: shellSleepCommand(50*time.Millisecond, 0),
		Restart: &RestartConfig{
			Policy: RestartOnExit,
			Delay:  500,
		},
	}); err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}

	waitUntil(t, func() bool {
		state := manager.RestartState(1)
		return manager.GetStatus(1) == StatusStopped && state.IsRestarting
	}, 2*time.Second)

	manager.Stop(1, true)
	time.Sleep(700 * time.Millisecond)

	if !manager.IsManuallyStopped(1) {
		t.Fatal("expected process to be manually stopped")
	}
	if state := manager.RestartState(1); state.IsRestarting {
		t.Fatalf("expected restarting state to clear, got %+v", state)
	}
	if got := countStartLogs(logBuffer, 1); got != 1 {
		t.Fatalf("expected pending restart to be canceled, got %d starts", got)
	}
}

func TestRestartClearsManualStop(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	if err := manager.StartCommand(CommandInfo{
		ID:      1,
		Name:    "long",
		Command: "sleep 10",
	}); err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}

	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusRunning }, time.Second)

	manager.Stop(1, true)
	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusStopped }, 2*time.Second)

	if !manager.IsManuallyStopped(1) {
		t.Fatal("expected process to be manually stopped")
	}

	manager.Restart(1, true)
	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusRunning }, 2*time.Second)

	if manager.IsManuallyStopped(1) {
		t.Fatal("expected manual stop flag to be cleared after restart")
	}
}

func intPtr(value int) *int {
	return &value
}
