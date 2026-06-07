package main

import (
	"testing"
	"time"
)

func TestLogsForProcessIDsMergesByTimestamp(t *testing.T) {
	logBuffer := NewLogBuffer()
	now := time.Now()

	logBuffer.mu.Lock()
	logBuffer.buffers[0] = []LogEntry{
		{Line: "b", ProcessID: 0, HasID: true, Timestamp: now.Add(time.Second)},
		{Line: "d", ProcessID: 0, HasID: true, Timestamp: now.Add(3 * time.Second)},
	}
	logBuffer.buffers[1] = []LogEntry{
		{Line: "a", ProcessID: 1, HasID: true, Timestamp: now},
		{Line: "c", ProcessID: 1, HasID: true, Timestamp: now.Add(2 * time.Second)},
	}
	logBuffer.mu.Unlock()

	logs := logBuffer.LogsForProcessIDs([]int{0, 1})
	if len(logs) != 4 {
		t.Fatalf("expected 4 logs, got %d", len(logs))
	}
	for i, want := range []string{"a", "b", "c", "d"} {
		if logs[i].Line != want {
			t.Fatalf("log %d: expected %q, got %q", i, want, logs[i].Line)
		}
	}
}
