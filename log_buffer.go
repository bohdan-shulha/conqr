package main

import (
	"slices"
	"sync"
	"time"
)

const maxLinesPerProcess = 1000

type LogEntry struct {
	Line      string
	Source    LogSource
	Timestamp time.Time
	ProcessID int
	HasID     bool
	IsSystem  bool
}

type LogBuffer struct {
	mu      sync.RWMutex
	buffers map[int][]LogEntry
	unified []LogEntry
	max     int
}

func NewLogBuffer() *LogBuffer {
	return &LogBuffer{
		buffers: make(map[int][]LogEntry),
		max:     maxLinesPerProcess,
	}
}

func (l *LogBuffer) Add(processID int, line string, source LogSource, isSystem bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Line:      line,
		Source:    source,
		Timestamp: time.Now(),
		ProcessID: processID,
		HasID:     true,
		IsSystem:  isSystem,
	}

	l.buffers[processID] = append(l.buffers[processID], entry)
	if len(l.buffers[processID]) > l.max {
		l.buffers[processID] = l.buffers[processID][1:]
	}

	l.unified = append(l.unified, entry)
	if len(l.unified) > l.max*10 {
		l.unified = l.unified[1:]
	}
}

func (l *LogBuffer) Logs(processID int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]LogEntry(nil), l.buffers[processID]...)
}

func (l *LogBuffer) UnifiedLogs() []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]LogEntry(nil), l.unified...)
}

func (l *LogBuffer) LogsForProcessIDs(ids []int) []LogEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()

	merged := make([]LogEntry, 0)
	for _, id := range ids {
		merged = append(merged, l.buffers[id]...)
	}
	slices.SortFunc(merged, func(a, b LogEntry) int {
		return a.Timestamp.Compare(b.Timestamp)
	})
	return merged
}

func (l *LogBuffer) Clear(processID *int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if processID != nil {
		delete(l.buffers, *processID)
		return
	}
	l.buffers = make(map[int][]LogEntry)
	l.unified = nil
}
