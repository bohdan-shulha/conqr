package main

import (
	"fmt"
	"os"
	"regexp"
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

func logContains(buffer *LogBuffer, processID int, needle string) bool {
	for _, entry := range buffer.Logs(processID) {
		if strings.Contains(entry.Line, needle) {
			return true
		}
	}
	return false
}

func TestStartAllWaitsForDependencyReadyLine(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	manager.StartAll([]CommandInfo{
		{
			ID:      1,
			Name:    "core",
			Command: "sleep 0.4; echo READY; sleep 5",
			Ready:   regexp.MustCompile("READY"),
		},
		{
			ID:        2,
			Name:      "api",
			Command:   "sleep 5",
			DependsOn: []string{"core"},
		},
	})

	time.Sleep(100 * time.Millisecond)
	if got := countStartLogs(logBuffer, 2); got != 0 {
		t.Fatalf("expected dependent to wait, got %d start logs", got)
	}
	if readiness := manager.Readiness(2); !readiness.Waiting {
		t.Fatalf("expected dependent to be waiting, got %+v", readiness)
	}
	if readiness := manager.Readiness(1); !readiness.Building {
		t.Fatalf("expected dependency to be building, got %+v", readiness)
	}

	waitUntil(t, func() bool { return manager.GetStatus(2) == StatusRunning }, 2*time.Second)

	if readiness := manager.Readiness(1); readiness.Building {
		t.Fatalf("expected dependency to leave building state, got %+v", readiness)
	}
	if readiness := manager.Readiness(2); readiness.Waiting {
		t.Fatalf("expected dependent to leave waiting state, got %+v", readiness)
	}
}

func TestStartAllStartsDependentAfterZeroExit(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	manager.StartAll([]CommandInfo{
		{ID: 1, Name: "emails", Command: shellSleepCommand(200*time.Millisecond, 0)},
		{ID: 2, Name: "web", Command: "sleep 5", DependsOn: []string{"emails"}},
	})

	time.Sleep(50 * time.Millisecond)
	if got := countStartLogs(logBuffer, 2); got != 0 {
		t.Fatalf("expected dependent to wait, got %d start logs", got)
	}

	waitUntil(t, func() bool { return manager.GetStatus(2) == StatusRunning }, 2*time.Second)
}

func TestStartAllStartsDependentAfterReadyTimeout(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	manager.StartAll([]CommandInfo{
		{ID: 1, Name: "core", Command: "sleep 10", Ready: regexp.MustCompile("NEVER")},
		{
			ID:           2,
			Name:         "api",
			Command:      "sleep 5",
			DependsOn:    []string{"core"},
			ReadyTimeout: 300 * time.Millisecond,
		},
	})

	time.Sleep(100 * time.Millisecond)
	if got := countStartLogs(logBuffer, 2); got != 0 {
		t.Fatalf("expected dependent to wait, got %d start logs", got)
	}

	waitUntil(t, func() bool { return manager.GetStatus(2) == StatusRunning }, 2*time.Second)

	if !logContains(logBuffer, 2, "core is not ready") {
		t.Fatal("expected a timeout warning in the dependent log")
	}
}

func TestReadyTimeoutUnblocksEveryWaiter(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	manager.StartAll([]CommandInfo{
		{ID: 1, Name: "core", Command: "sleep 10", Ready: regexp.MustCompile("NEVER")},
		{ID: 2, Name: "api", Command: "sleep 5", DependsOn: []string{"core"}, ReadyTimeout: 300 * time.Millisecond},
		{ID: 3, Name: "web", Command: "sleep 5", DependsOn: []string{"core"}, ReadyTimeout: 10 * time.Second},
	})

	waitUntil(t, func() bool {
		return manager.GetStatus(2) == StatusRunning && manager.GetStatus(3) == StatusRunning
	}, 2*time.Second)
}

func TestKillAllReleasesWaitingDependent(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)

	manager.StartAll([]CommandInfo{
		{ID: 1, Name: "core", Command: "sleep 10", Ready: regexp.MustCompile("NEVER")},
		{ID: 2, Name: "api", Command: "sleep 5", DependsOn: []string{"core"}},
	})

	time.Sleep(50 * time.Millisecond)
	manager.KillAll()
	time.Sleep(200 * time.Millisecond)

	if got := countStartLogs(logBuffer, 2); got != 0 {
		t.Fatalf("expected waiting dependent to never start, got %d start logs", got)
	}
}

func TestStopCancelsPendingDependentStart(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	manager.StartAll([]CommandInfo{
		{
			ID:      1,
			Name:    "core",
			Command: "sleep 0.3; echo READY; sleep 5",
			Ready:   regexp.MustCompile("READY"),
		},
		{ID: 2, Name: "api", Command: "sleep 5", DependsOn: []string{"core"}},
	})

	manager.Stop(2, true)
	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusRunning }, time.Second)
	time.Sleep(500 * time.Millisecond)

	if got := countStartLogs(logBuffer, 2); got != 0 {
		t.Fatalf("expected stopped dependent to never start, got %d start logs", got)
	}
}

func TestBusyPatternReturnsProcessToBuilding(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	if err := manager.StartCommand(CommandInfo{
		ID:      1,
		Name:    "core",
		Command: "echo READY; sleep 0.3; echo REBUILD; sleep 5",
		Ready:   regexp.MustCompile("READY"),
		Busy:    regexp.MustCompile("REBUILD"),
	}); err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}

	waitUntil(t, func() bool { return !manager.Readiness(1).Building }, time.Second)
	waitUntil(t, func() bool { return manager.Readiness(1).Building }, 2*time.Second)
}

func TestMarkReadyIsIdempotent(t *testing.T) {
	manager := NewProcessManager(NewLogBuffer())

	manager.markReady(1)
	manager.markReady(1)

	select {
	case <-manager.readySignal(1):
	default:
		t.Fatal("expected the ready signal to be closed")
	}
}

func TestRestartPreservesCommandConfiguration(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	if err := manager.StartCommand(CommandInfo{
		ID:      1,
		Name:    "core",
		Command: "sleep 5",
		Group:   "services",
		Ready:   regexp.MustCompile("READY"),
	}); err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}

	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusRunning }, time.Second)
	manager.Restart(1, true)
	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusRunning }, 2*time.Second)

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.processes[1].Group != "services" {
		t.Fatalf("expected group to survive a restart, got %q", manager.processes[1].Group)
	}
	if manager.processes[1].Ready == nil {
		t.Fatal("expected ready pattern to survive a restart")
	}
}

func TestStaleRunClosesExitChannel(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	if err := manager.StartCommand(CommandInfo{ID: 1, Name: "core", Command: "sleep 10"}); err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}
	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusRunning }, time.Second)

	manager.mu.Lock()
	previous := manager.processes[1]
	manager.mu.Unlock()

	manager.Restart(1, true)
	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusRunning }, 2*time.Second)

	manager.mu.Lock()
	replaced := manager.processes[1] != previous
	manager.mu.Unlock()
	if !replaced {
		t.Fatal("expected the restart to replace the process entry")
	}

	select {
	case <-previous.exited:
	case <-time.After(time.Second):
		t.Fatal("expected the superseded run to close its exit channel")
	}
	if isLive(previous) {
		t.Fatal("expected the superseded run to report as not live")
	}
}

func TestStopReturnsPromptlyForRunningProcess(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	if err := manager.StartCommand(CommandInfo{ID: 1, Name: "core", Command: "sleep 10"}); err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}
	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusRunning }, time.Second)

	start := time.Now()
	manager.Stop(1, true)
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("expected Stop to return once the process exits, took %s", elapsed)
	}
}

func TestRestartStartsDependentStoppedWhileWaiting(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	manager.StartAll([]CommandInfo{
		{ID: 1, Name: "core", Command: "sleep 10", Ready: regexp.MustCompile("NEVER")},
		{ID: 2, Name: "api", Command: "sleep 5", DependsOn: []string{"core"}, ReadyTimeout: 10 * time.Second},
	})

	time.Sleep(100 * time.Millisecond)
	manager.Stop(2, true)
	if !manager.IsManuallyStopped(2) {
		t.Fatal("expected the waiting dependent to be manually stopped")
	}
	if got := countStartLogs(logBuffer, 2); got != 0 {
		t.Fatalf("expected the stopped dependent to never start, got %d start logs", got)
	}

	manager.Restart(2, true)
	waitUntil(t, func() bool { return manager.GetStatus(2) == StatusRunning }, 2*time.Second)

	if manager.IsManuallyStopped(2) {
		t.Fatal("expected the restart to clear the manually stopped flag")
	}
	if readiness := manager.Readiness(2); readiness.Waiting {
		t.Fatalf("expected the waiting flag to clear, got %+v", readiness)
	}
	if got := countStartLogs(logBuffer, 2); got != 1 {
		t.Fatalf("expected exactly 1 start log, got %d", got)
	}
	if state := manager.RestartState(2); state.RestartCount != 0 {
		t.Fatalf("expected no restart count for a process that never ran, got %d", state.RestartCount)
	}
}

func TestRestartWhileWaitingStartsDependentOnlyOnce(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	manager.StartAll([]CommandInfo{
		{ID: 1, Name: "core", Command: "sleep 10", Ready: regexp.MustCompile("NEVER")},
		{ID: 2, Name: "api", Command: "sleep 5", DependsOn: []string{"core"}, ReadyTimeout: 200 * time.Millisecond},
	})

	manager.Restart(2, true)
	waitUntil(t, func() bool { return manager.GetStatus(2) == StatusRunning }, 2*time.Second)

	time.Sleep(500 * time.Millisecond)

	if got := countStartLogs(logBuffer, 2); got != 1 {
		t.Fatalf("expected the waiter to skip a started process, got %d start logs", got)
	}
}

func TestStopClearsWaitingFlagForDependent(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	manager.StartAll([]CommandInfo{
		{
			ID:      1,
			Name:    "core",
			Command: "sleep 0.3; echo READY; sleep 5",
			Ready:   regexp.MustCompile("READY"),
		},
		{ID: 2, Name: "api", Command: "sleep 5", DependsOn: []string{"core"}},
	})

	manager.Stop(2, true)
	waitUntil(t, func() bool { return !manager.Readiness(2).Waiting }, 2*time.Second)

	if got := countStartLogs(logBuffer, 2); got != 0 {
		t.Fatalf("expected the stopped dependent to never start, got %d start logs", got)
	}
	if !logContains(logBuffer, 2, "Stop initiated") {
		t.Fatal("expected a stop message in the waiting dependent log")
	}
}

func TestRestartIgnoresUnknownProcess(t *testing.T) {
	manager := NewProcessManager(NewLogBuffer())

	manager.Restart(99, true)

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.restartInFlight[99] {
		t.Fatal("expected the restart claim to be released for an unknown process")
	}
}

func TestStartCommandStopsProcessStoppedWhileStarting(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	manager.mu.Lock()
	manager.manuallyStopped[1] = true
	manager.mu.Unlock()

	if err := manager.StartCommand(CommandInfo{ID: 1, Name: "api", Command: "sleep 10"}); err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}

	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusStopped }, 2*time.Second)

	manager.mu.Lock()
	info := manager.processes[1]
	manager.mu.Unlock()
	if isLive(info) {
		t.Fatal("expected the process to be killed when it starts while stopped")
	}
}

func TestStartCommandKeepsProcessThatIsNotStopped(t *testing.T) {
	logBuffer := NewLogBuffer()
	manager := NewProcessManager(logBuffer)
	defer manager.KillAll()

	if err := manager.StartCommand(CommandInfo{ID: 1, Name: "api", Command: "sleep 10"}); err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}

	waitUntil(t, func() bool { return manager.GetStatus(1) == StatusRunning }, time.Second)
	time.Sleep(200 * time.Millisecond)

	if status := manager.GetStatus(1); status != StatusRunning {
		t.Fatalf("expected the process to keep running, got %s", status)
	}
}
