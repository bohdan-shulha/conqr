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
	exited chan struct{}
}

type RestartState struct {
	IsRestarting bool
	RestartCount int
	CrashCount   int
}

type Readiness struct {
	Waiting  bool
	Building bool
}

type readyState struct {
	signal     chan struct{}
	ready      bool
	building   bool
	waiting    bool
	buildStart time.Time
}

type ProcessManager struct {
	mu                      sync.Mutex
	processes               map[int]*processInfo
	logBuffer               *LogBuffer
	shuttingDown            bool
	shutdownSignal          chan struct{}
	commands                map[int]CommandInfo
	readyStates             map[int]*readyState
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
		shutdownSignal:          make(chan struct{}),
		commands:                make(map[int]CommandInfo),
		readyStates:             make(map[int]*readyState),
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
	idsByName := make(map[string]int, len(commands))
	for _, command := range commands {
		idsByName[command.Name] = command.ID
	}

	pm.mu.Lock()
	for _, command := range commands {
		pm.commands[command.ID] = command
		if len(command.DependsOnCommands) > 0 {
			pm.readyStateFor(command.ID).waiting = true
		}
	}
	pm.mu.Unlock()

	for _, command := range commands {
		if len(command.DependsOnCommands) == 0 {
			pm.StartCommand(command)
			continue
		}

		names := make([]string, 0, len(command.DependsOnCommands))
		ids := make([]int, 0, len(command.DependsOnCommands))
		for _, name := range command.DependsOnCommands {
			if id, ok := idsByName[name]; ok {
				names = append(names, name)
				ids = append(ids, id)
			}
		}
		go pm.startWhenReady(command, ids, names)
	}
}

func (pm *ProcessManager) startWhenReady(commandInfo CommandInfo, dependencies []int, names []string) {
	timeout := commandInfo.ReadyTimeout
	if timeout <= 0 {
		timeout = defaultReadyTimeout
	}

	pm.logBuffer.Add(commandInfo.ID, "› Waiting for "+strings.Join(commandInfo.DependsOn, ", "), SourceStdout, true)

	for index, dependencyID := range dependencies {
		select {
		case <-pm.readySignal(dependencyID):
		case <-pm.shutdownSignal:
			return
		case <-time.After(timeout):
			pm.logBuffer.Add(commandInfo.ID, "! "+names[index]+" is not ready, starting anyway", SourceStderr, true)
			pm.markReady(dependencyID)
		}
	}

	pm.markWaiting(commandInfo.ID, false)
	if !pm.claimStart(commandInfo.ID) {
		return
	}
	_ = pm.StartCommand(commandInfo)
	pm.releaseStart(commandInfo.ID)
}

func (pm *ProcessManager) claimStart(processID int) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.shuttingDown || pm.manuallyStopped[processID] ||
		pm.processes[processID] != nil || pm.restartInFlight[processID] {
		return false
	}
	pm.restartInFlight[processID] = true
	return true
}

func (pm *ProcessManager) releaseStart(processID int) {
	pm.mu.Lock()
	delete(pm.restartInFlight, processID)
	pm.mu.Unlock()
}

func (pm *ProcessManager) readyStateFor(processID int) *readyState {
	state := pm.readyStates[processID]
	if state == nil {
		state = &readyState{signal: make(chan struct{})}
		pm.readyStates[processID] = state
	}
	return state
}

func (pm *ProcessManager) readySignal(processID int) chan struct{} {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.readyStateFor(processID).signal
}

func (pm *ProcessManager) markReady(processID int) time.Duration {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	state := pm.readyStateFor(processID)
	elapsed := time.Duration(0)
	if state.building {
		elapsed = time.Since(state.buildStart)
		state.building = false
	}
	if state.ready {
		return elapsed
	}
	state.ready = true
	close(state.signal)
	return elapsed
}

func (pm *ProcessManager) markBuilding(processID int) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	state := pm.readyStateFor(processID)
	if state.building {
		return false
	}
	state.building = true
	state.buildStart = time.Now()
	return true
}

func (pm *ProcessManager) markWaiting(processID int, waiting bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.readyStateFor(processID).waiting = waiting
}

func (pm *ProcessManager) Readiness(processID int) Readiness {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	state := pm.readyStates[processID]
	if state == nil {
		return Readiness{}
	}
	return Readiness{Waiting: state.waiting, Building: state.building}
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
		failed := &processInfo{
			CommandInfo: commandInfo,
			Status:      StatusError,
			Cmd:         cmd,
			RunID:       runID,
			exited:      make(chan struct{}),
		}
		close(failed.exited)
		pm.mu.Lock()
		pm.processes[commandInfo.ID] = failed
		pm.mu.Unlock()
		pm.markReady(commandInfo.ID)
		return err
	}

	pm.logBuffer.Add(commandInfo.ID, "› Service starting", SourceStdout, true)

	info := &processInfo{
		CommandInfo: commandInfo,
		Status:      StatusRunning,
		Cmd:         cmd,
		RunID:       runID,
		PID:         cmd.Process.Pid,
		exited:      make(chan struct{}),
	}

	pm.mu.Lock()
	pm.processes[commandInfo.ID] = info
	state := pm.readyStateFor(commandInfo.ID)
	state.building = commandInfo.Ready != nil
	state.buildStart = time.Now()
	stoppedWhileStarting := pm.manuallyStopped[commandInfo.ID]
	pm.mu.Unlock()

	go pm.consumePipe(commandInfo.ID, stdout, SourceStdout)
	go pm.consumePipe(commandInfo.ID, stderr, SourceStderr)
	go pm.waitForExit(info)

	if stoppedWhileStarting {
		pm.stopAfterStart(info)
	}

	return nil
}

func (pm *ProcessManager) stopAfterStart(info *processInfo) {
	pm.mu.Lock()
	stop := pm.manuallyStopped[info.ID] && pm.processes[info.ID] == info
	pm.mu.Unlock()
	if stop {
		pm.killOne(info)
	}
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
			pm.checkReadiness(processID, line)
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

func (pm *ProcessManager) waitForExit(info *processInfo) {
	cmd := info.Cmd
	err := cmd.Wait()
	close(info.exited)
	code := exitCode(err)
	processID := info.ID

	pm.mu.Lock()
	wasIntentional := pm.intentionalExitCommands[cmd]
	delete(pm.intentionalExitCommands, cmd)
	if pm.processes[processID] != info {
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

	if code != nil && *code == 0 {
		if elapsed := pm.markReady(processID); elapsed > 0 {
			pm.logBuffer.Add(processID, formatReadyMessage(elapsed), SourceStdout, true)
		}
	}

	willRestart := false
	if restartConfig != nil && !wasIntentional && !shuttingDown && !manuallyStopped {
		if restartConfig.Policy == RestartOnExit || (restartConfig.Policy == RestartOnError && hasNonZeroExitCode) {
			pm.scheduleRestart(processID, restartConfig.Delay, code, info.RunID)
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

func formatReadyMessage(elapsed time.Duration) string {
	return fmt.Sprintf("› Ready in %.1fs", elapsed.Seconds())
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
	if !pm.shuttingDown {
		pm.shuttingDown = true
		close(pm.shutdownSignal)
	}
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
		if isLive(info) {
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
	live := isLive(info)
	pm.mu.Unlock()

	if manual {
		message := "› Stop initiated"
		if live {
			message = "› Stop initiated, SIGTERM signal sent"
		}
		pm.logBuffer.Add(processID, message, SourceStdout, true)
	}

	if info == nil {
		pm.mu.Lock()
		delete(pm.stopInFlight, processID)
		pm.mu.Unlock()
		return
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
		commandInfo, known := pm.commands[processID]
		if !known {
			delete(pm.restartInFlight, processID)
			pm.mu.Unlock()
			return
		}
		pm.readyStateFor(processID).waiting = false
		pm.mu.Unlock()

		if manual {
			pm.logBuffer.Add(processID, "› Restart initiated", SourceStdout, true)
		}
		_ = pm.StartCommand(commandInfo)
		pm.releaseStart(processID)
		return
	}
	if pm.isRestarting[processID] {
		pm.isRestarting[processID] = false
	}
	live := isLive(info)
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
	replacement := info.CommandInfo
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

func isLive(info *processInfo) bool {
	if info == nil || info.Cmd == nil || info.Cmd.Process == nil {
		return false
	}
	select {
	case <-info.exited:
		return false
	default:
		return true
	}
}

func (pm *ProcessManager) killOne(info *processInfo) {
	if !isLive(info) {
		return
	}

	pm.killProcess(info, terminateSignal)
	waitForProcessExit(info, time.Second)
	if isLive(info) {
		pm.killProcess(info, killSignal)
		waitForProcessExit(info, 500*time.Millisecond)
	}
}

func waitForProcessExit(info *processInfo, timeout time.Duration) {
	if info == nil || info.exited == nil {
		return
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	select {
	case <-info.exited:
	case <-deadline.C:
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

func (pm *ProcessManager) checkReadiness(processID int, line string) {
	pm.mu.Lock()
	info := pm.processes[processID]
	if info == nil || (info.Ready == nil && info.Busy == nil) {
		pm.mu.Unlock()
		return
	}
	ready, busy := info.Ready, info.Busy
	pm.mu.Unlock()

	plain := stripANSI(line)
	if busy != nil && busy.MatchString(plain) {
		if pm.markBuilding(processID) {
			pm.logBuffer.Add(processID, "› Build started: "+plain, SourceStdout, true)
		}
		return
	}
	if ready != nil && ready.MatchString(plain) {
		if elapsed := pm.markReady(processID); elapsed > 0 {
			pm.logBuffer.Add(processID, formatReadyMessage(elapsed), SourceStdout, true)
		}
	}
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
	} else if !hasRecentError && info.Status == StatusError && isLive(info) {
		info.Status = StatusRunning
	}
}
