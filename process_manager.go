package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

type processInfo struct {
	CommandInfo
	Status ProcessStatus
	Cmd    *exec.Cmd
	RunID  int
	PID    int
}

type RestartState struct {
	IsRestarting bool
	RestartCount int
	CrashCount   int
}

type ProcessManager struct {
	mu                      sync.Mutex
	processes               map[int]*processInfo
	logBuffer               *LogBuffer
	shuttingDown            bool
	restartTimers           map[int]*time.Timer
	restartCounts           map[int]int
	crashCounts             map[int]int
	isRestarting            map[int]bool
	restartInFlight         map[int]bool
	stopInFlight            map[int]bool
	manuallyStopped         map[int]bool
	intentionalExitCommands map[*exec.Cmd]bool
}

func NewProcessManager(logBuffer *LogBuffer) *ProcessManager {
	return &ProcessManager{
		processes:               make(map[int]*processInfo),
		logBuffer:               logBuffer,
		restartTimers:           make(map[int]*time.Timer),
		restartCounts:           make(map[int]int),
		crashCounts:             make(map[int]int),
		isRestarting:            make(map[int]bool),
		restartInFlight:         make(map[int]bool),
		stopInFlight:            make(map[int]bool),
		manuallyStopped:         make(map[int]bool),
		intentionalExitCommands: make(map[*exec.Cmd]bool),
	}
}

func (pm *ProcessManager) StartAll(commands []CommandInfo) {
	for _, command := range commands {
		pm.StartCommand(command)
	}
}

func (pm *ProcessManager) StartCommand(commandInfo CommandInfo) error {
	pm.mu.Lock()
	if pm.shuttingDown {
		pm.mu.Unlock()
		return nil
	}
	pm.mu.Unlock()

	cmd := shellCommand(commandInfo.Command)
	cmd.Stdin = nil
	cmd.SysProcAttr = processAttributes()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	pm.mu.Lock()
	if pm.shuttingDown {
		pm.mu.Unlock()
		return nil
	}
	runID := 1
	if existing := pm.processes[commandInfo.ID]; existing != nil {
		runID = existing.RunID + 1
	}
	pm.mu.Unlock()

	if err := cmd.Start(); err != nil {
		pm.logBuffer.Add(commandInfo.ID, "Process error: "+err.Error(), SourceStderr, true)
		pm.mu.Lock()
		pm.processes[commandInfo.ID] = &processInfo{
			CommandInfo: commandInfo,
			Status:      StatusError,
			Cmd:         cmd,
			RunID:       runID,
		}
		pm.mu.Unlock()
		return err
	}

	pm.logBuffer.Add(commandInfo.ID, "› Service starting", SourceStdout, true)

	info := &processInfo{
		CommandInfo: commandInfo,
		Status:      StatusRunning,
		Cmd:         cmd,
		RunID:       runID,
		PID:         cmd.Process.Pid,
	}

	pm.mu.Lock()
	pm.processes[commandInfo.ID] = info
	pm.mu.Unlock()

	go pm.consumePipe(commandInfo.ID, stdout, SourceStdout)
	go pm.consumePipe(commandInfo.ID, stderr, SourceStderr)
	go pm.waitForExit(commandInfo.ID, cmd, runID)

	return nil
}

func shellCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command)
	}
	return exec.Command("sh", "-c", command)
}

func (pm *ProcessManager) consumePipe(processID int, reader io.Reader, source LogSource) {
	buffered := bufio.NewReader(reader)
	for {
		line, err := buffered.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line != "" {
			pm.logBuffer.Add(processID, line, source, false)
			pm.checkRecentErrors(processID)
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				pm.logBuffer.Add(processID, "Process stream error: "+err.Error(), SourceStderr, true)
			}
			return
		}
	}
}

func (pm *ProcessManager) waitForExit(processID int, cmd *exec.Cmd, runID int) {
	err := cmd.Wait()
	code := exitCode(err)

	pm.mu.Lock()
	wasIntentional := pm.intentionalExitCommands[cmd]
	delete(pm.intentionalExitCommands, cmd)
	info := pm.processes[processID]
	if info == nil || info.Cmd != cmd {
		pm.mu.Unlock()
		return
	}
	info.Status = StatusStopped
	if cmd.Process != nil {
		info.PID = cmd.Process.Pid
	}
	hasNonZeroExitCode := code != nil && *code != 0
	if hasNonZeroExitCode && !wasIntentional {
		pm.crashCounts[processID]++
	}
	restartConfig := info.Restart
	shuttingDown := pm.shuttingDown
	manuallyStopped := pm.manuallyStopped[processID]
	pm.mu.Unlock()

	willRestart := false
	if restartConfig != nil && !wasIntentional && !shuttingDown && !manuallyStopped {
		if restartConfig.Policy == RestartOnExit || (restartConfig.Policy == RestartOnError && hasNonZeroExitCode) {
			pm.scheduleRestart(processID, restartConfig.Delay, code, runID)
			willRestart = true
		}
	}

	if !willRestart {
		pm.logBuffer.Add(processID, formatExitMessage(code), SourceStdout, true)
	}
}

func exitCode(err error) *int {
	code := 0
	if err == nil {
		return &code
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
		return &code
	}
	return nil
}

func formatExitMessage(code *int) string {
	if code != nil && *code == 0 {
		return "• Process exited with code 0"
	}
	if code == nil {
		return "× Process exited with code null"
	}
	return fmt.Sprintf("× Process exited with code %d", *code)
}

func (pm *ProcessManager) GetStatus(processID int) ProcessStatus {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if info := pm.processes[processID]; info != nil {
		return info.Status
	}
	return StatusUnknown
}

func (pm *ProcessManager) GetAllStatuses() map[int]ProcessStatus {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	result := make(map[int]ProcessStatus, len(pm.processes))
	for id, info := range pm.processes {
		result[id] = info.Status
	}
	return result
}

func (pm *ProcessManager) RestartState(processID int) RestartState {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return RestartState{
		IsRestarting: pm.isRestarting[processID],
		RestartCount: pm.restartCounts[processID],
		CrashCount:   pm.crashCounts[processID],
	}
}

func (pm *ProcessManager) KillAll() {
	infos := pm.beginShutdown()

	var wg sync.WaitGroup
	for _, info := range infos {
		wg.Add(1)
		go func(info *processInfo) {
			defer wg.Done()
			pm.killOne(info)
		}(info)
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1500 * time.Millisecond):
		for _, info := range infos {
			pm.killProcess(info, killSignal)
		}
	}
}

func (pm *ProcessManager) KillAllForExit(timeout time.Duration) {
	infos := pm.beginShutdown()
	for _, info := range infos {
		pm.killProcess(info, terminateSignal)
	}

	deadline := time.NewTimer(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()

	for {
		if allExited(infos) {
			return
		}
		select {
		case <-deadline.C:
			for _, info := range infos {
				pm.killProcess(info, killSignal)
			}
			return
		case <-ticker.C:
		}
	}
}

func (pm *ProcessManager) beginShutdown() []*processInfo {
	pm.mu.Lock()
	pm.shuttingDown = true
	for _, timer := range pm.restartTimers {
		timer.Stop()
	}
	pm.restartTimers = make(map[int]*time.Timer)
	for id := range pm.isRestarting {
		pm.isRestarting[id] = false
	}
	pm.manuallyStopped = make(map[int]bool)
	infos := make([]*processInfo, 0, len(pm.processes))
	for _, info := range pm.processes {
		infos = append(infos, info)
	}
	pm.mu.Unlock()

	return infos
}

func allExited(infos []*processInfo) bool {
	for _, info := range infos {
		if info != nil && isLive(info.Cmd) {
			return false
		}
	}
	return true
}

func (pm *ProcessManager) IsManuallyStopped(processID int) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.manuallyStopped[processID]
}

func (pm *ProcessManager) Stop(processID int, manual bool) {
	pm.mu.Lock()
	if pm.shuttingDown {
		pm.mu.Unlock()
		return
	}
	if pm.stopInFlight[processID] {
		pm.mu.Unlock()
		return
	}
	pm.stopInFlight[processID] = true

	if timer := pm.restartTimers[processID]; timer != nil {
		timer.Stop()
		delete(pm.restartTimers, processID)
	}
	pm.isRestarting[processID] = false
	pm.manuallyStopped[processID] = true

	info := pm.processes[processID]
	if info == nil {
		delete(pm.stopInFlight, processID)
		pm.mu.Unlock()
		return
	}
	live := isLive(info.Cmd)
	pm.mu.Unlock()

	if manual {
		message := "› Stop initiated"
		if live {
			message = "› Stop initiated, SIGTERM signal sent"
		}
		pm.logBuffer.Add(processID, message, SourceStdout, true)
	}

	if live {
		pm.killOne(info)
	}

	pm.mu.Lock()
	delete(pm.stopInFlight, processID)
	pm.mu.Unlock()
}

func (pm *ProcessManager) Restart(processID int, manual bool) {
	pm.mu.Lock()
	if pm.shuttingDown {
		pm.mu.Unlock()
		return
	}
	if pm.restartInFlight[processID] {
		pm.mu.Unlock()
		return
	}
	pm.restartInFlight[processID] = true
	delete(pm.manuallyStopped, processID)

	if timer := pm.restartTimers[processID]; timer != nil {
		timer.Stop()
		delete(pm.restartTimers, processID)
	}

	info := pm.processes[processID]
	if info == nil {
		delete(pm.restartInFlight, processID)
		pm.mu.Unlock()
		return
	}
	if pm.isRestarting[processID] {
		pm.isRestarting[processID] = false
	}
	live := isLive(info.Cmd)
	pm.mu.Unlock()

	if manual {
		message := "› Restart initiated"
		if live {
			message = "› Restart initiated, SIGTERM signal sent"
		}
		pm.logBuffer.Add(processID, message, SourceStdout, true)
	}

	if live {
		pm.killOne(info)
	}

	pm.mu.Lock()
	if pm.shuttingDown {
		delete(pm.restartInFlight, processID)
		pm.mu.Unlock()
		return
	}
	pm.restartCounts[processID]++
	pm.isRestarting[processID] = false
	replacement := CommandInfo{
		ID:      info.ID,
		Name:    info.Name,
		Command: info.Command,
		Restart: info.Restart,
	}
	pm.mu.Unlock()

	_ = pm.StartCommand(replacement)

	pm.mu.Lock()
	delete(pm.restartInFlight, processID)
	pm.mu.Unlock()
}

func (pm *ProcessManager) scheduleRestart(processID int, delay int, code *int, expectedRunID int) {
	pm.mu.Lock()
	if pm.shuttingDown || pm.manuallyStopped[processID] {
		pm.mu.Unlock()
		return
	}
	if existing := pm.restartTimers[processID]; existing != nil {
		existing.Stop()
	}
	pm.isRestarting[processID] = true
	pm.mu.Unlock()

	pm.logBuffer.Add(processID, formatExitMessageWithRestart(code, delay), SourceStdout, true)

	timer := time.AfterFunc(time.Duration(delay)*time.Millisecond, func() {
		pm.mu.Lock()
		if pm.shuttingDown {
			delete(pm.restartTimers, processID)
			pm.isRestarting[processID] = false
			pm.mu.Unlock()
			return
		}
		delete(pm.restartTimers, processID)
		current := pm.processes[processID]
		if current == nil || current.RunID != expectedRunID || current.Status != StatusStopped {
			if current != nil && current.RunID == expectedRunID && pm.isRestarting[processID] {
				pm.isRestarting[processID] = false
			}
			pm.mu.Unlock()
			return
		}
		pm.mu.Unlock()
		pm.Restart(processID, false)
	})

	pm.mu.Lock()
	pm.restartTimers[processID] = timer
	pm.mu.Unlock()
}

func formatExitMessageWithRestart(code *int, delay int) string {
	delaySeconds := float64(delay) / 1000
	if code != nil && *code == 0 {
		return fmt.Sprintf("• Process exited with code 0 › %.1fs", delaySeconds)
	}
	if code == nil {
		return fmt.Sprintf("× Process exited with code null › %.1fs", delaySeconds)
	}
	return fmt.Sprintf("× Process exited with code %d › %.1fs", *code, delaySeconds)
}

func isLive(cmd *exec.Cmd) bool {
	return cmd != nil && cmd.Process != nil && cmd.ProcessState == nil
}

func (pm *ProcessManager) killOne(info *processInfo) {
	if info == nil || !isLive(info.Cmd) {
		return
	}

	pm.killProcess(info, terminateSignal)
	waitForProcessExit(info.Cmd, time.Second)
	if isLive(info.Cmd) {
		pm.killProcess(info, killSignal)
		waitForProcessExit(info.Cmd, 500*time.Millisecond)
	}
}

func waitForProcessExit(cmd *exec.Cmd, timeout time.Duration) {
	deadline := time.NewTimer(timeout)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()

	for {
		if !isLive(cmd) {
			return
		}
		select {
		case <-deadline.C:
			return
		case <-ticker.C:
		}
	}
}

func (pm *ProcessManager) killProcess(info *processInfo, signal os.Signal) {
	if info == nil || info.Cmd == nil || info.Cmd.Process == nil {
		return
	}

	pm.mu.Lock()
	pm.intentionalExitCommands[info.Cmd] = true
	pm.mu.Unlock()

	pid := info.Cmd.Process.Pid
	if killProcessGroup(pid, signal) {
		return
	}
	_ = info.Cmd.Process.Signal(signal)
}

var (
	redANSIRegex   = regexp.MustCompile(`\x1b\[(31|91|38;5;1)m`)
	errorRegexList = []*regexp.Regexp{
		regexp.MustCompile(`(?i)SyntaxError`),
		regexp.MustCompile(`(?i)TypeError`),
		regexp.MustCompile(`(?i)ReferenceError`),
		regexp.MustCompile(`(?i)Error:`),
		regexp.MustCompile(`(?i)Error\s+at`),
		regexp.MustCompile(`(?i)FATAL`),
		regexp.MustCompile(`(?i)CRITICAL`),
		regexp.MustCompile(`(?i)failed`),
		regexp.MustCompile(`(?i)failure`),
		regexp.MustCompile(`(?i)cannot`),
		regexp.MustCompile(`(?i)uncaught`),
		regexp.MustCompile(`(?i)unhandled`),
		regexp.MustCompile(`^\s+at .+\(.+:\d+:\d+\)$`),
	}
)

func detectError(line string) bool {
	if redANSIRegex.MatchString(line) {
		return true
	}
	for _, pattern := range errorRegexList {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

func (pm *ProcessManager) checkRecentErrors(processID int) {
	pm.mu.Lock()
	info := pm.processes[processID]
	if info == nil || info.Status == StatusStopped || info.Status == StatusUnknown {
		pm.mu.Unlock()
		return
	}
	pm.mu.Unlock()

	logs := pm.logBuffer.Logs(processID)
	if len(logs) > 10 {
		logs = logs[len(logs)-10:]
	}

	hasRecentError := false
	for _, entry := range logs {
		if detectError(entry.Line) {
			hasRecentError = true
			break
		}
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()
	info = pm.processes[processID]
	if info == nil {
		return
	}
	if hasRecentError && info.Status == StatusRunning {
		info.Status = StatusError
	} else if !hasRecentError && info.Status == StatusError && isLive(info.Cmd) {
		info.Status = StatusRunning
	}
}
