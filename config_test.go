package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseConfigPreservesCommandOrderAndMergesRestart(t *testing.T) {
	content := []byte(`{
		"restart": { "policy": "on-error", "delay": 2000 },
		"commands": {
			"Dev": "npm run dev",
			"Worker": {
				"command": "npm run worker",
				"restart": { "policy": "on-exit" }
			}
		}
	}`)

	commands, defaultGroup, err := parseConfig(content)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if defaultGroup != "" {
		t.Fatalf("expected empty defaultGroup, got %q", defaultGroup)
	}
	if len(commands) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(commands))
	}
	if commands[0].Name != "Dev" || commands[0].ID != 0 {
		t.Fatalf("first command not preserved: %+v", commands[0])
	}
	if commands[1].Name != "Worker" || commands[1].ID != 1 {
		t.Fatalf("second command not preserved: %+v", commands[1])
	}
	if commands[0].Restart == nil || commands[0].Restart.Policy != RestartOnError || commands[0].Restart.Delay != 2000 {
		t.Fatalf("global restart not applied: %+v", commands[0].Restart)
	}
	if commands[1].Restart == nil || commands[1].Restart.Policy != RestartOnExit || commands[1].Restart.Delay != 2000 {
		t.Fatalf("per-process restart not merged: %+v", commands[1].Restart)
	}
}

func TestParseConfigMergesReadyAndBusyPatterns(t *testing.T) {
	content := []byte(`{
		"ready": "Found 0 errors",
		"busy": "File change detected",
		"commands": {
			"core": "tsc -w",
			"web": { "command": "vite dev", "ready": "ready in" },
			"tunnel": { "command": "cloudflared run", "ready": "" }
		}
	}`)

	commands, _, err := parseConfig(content)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if commands[0].Ready == nil || commands[0].Ready.String() != "Found 0 errors" {
		t.Fatalf("global ready not applied to string command: %+v", commands[0].Ready)
	}
	if commands[0].Busy == nil || commands[0].Busy.String() != "File change detected" {
		t.Fatalf("global busy not applied to string command: %+v", commands[0].Busy)
	}
	if commands[1].Ready == nil || commands[1].Ready.String() != "ready in" {
		t.Fatalf("per-process ready not merged: %+v", commands[1].Ready)
	}
	if commands[1].Busy == nil || commands[1].Busy.String() != "File change detected" {
		t.Fatalf("global busy not inherited: %+v", commands[1].Busy)
	}
	if commands[2].Ready != nil {
		t.Fatalf("empty ready must clear the global pattern, got %+v", commands[2].Ready)
	}
}

func TestParseConfigReadsDependsOnAndReadyTimeout(t *testing.T) {
	content := []byte(`{
		"readyTimeout": 500,
		"commands": {
			"core": "tsc -w",
			"api": { "command": "tsc -w -p api", "dependsOn": ["core"] }
		}
	}`)

	commands, _, err := parseConfig(content)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if len(commands[0].DependsOn) != 0 {
		t.Fatalf("expected no dependencies for core, got %v", commands[0].DependsOn)
	}
	if len(commands[1].DependsOn) != 1 || commands[1].DependsOn[0] != "core" {
		t.Fatalf("dependsOn not parsed: %v", commands[1].DependsOn)
	}
	if commands[1].ReadyTimeout != 500*time.Millisecond {
		t.Fatalf("expected 500ms readyTimeout, got %s", commands[1].ReadyTimeout)
	}
}

func TestParseConfigUsesDefaultReadyTimeout(t *testing.T) {
	content := []byte(`{ "commands": { "core": "tsc -w" } }`)

	commands, _, err := parseConfig(content)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if commands[0].ReadyTimeout != defaultReadyTimeout {
		t.Fatalf("expected default readyTimeout, got %s", commands[0].ReadyTimeout)
	}
}

func TestParseConfigRejectsUnknownDependency(t *testing.T) {
	content := []byte(`{
		"commands": {
			"core": "tsc -w",
			"api": { "command": "tsc -w -p api", "dependsOn": ["corr"] }
		}
	}`)

	_, _, err := parseConfig(content)
	if err == nil {
		t.Fatal("expected unknown dependency to be rejected")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("expected unknown command error, got %v", err)
	}
}

func TestParseConfigRejectsDependencyCycle(t *testing.T) {
	content := []byte(`{
		"commands": {
			"core": { "command": "tsc -w", "dependsOn": ["api"] },
			"db": { "command": "tsc -w -p db", "dependsOn": ["core"] },
			"api": { "command": "tsc -w -p api", "dependsOn": ["db"] }
		}
	}`)

	_, _, err := parseConfig(content)
	if err == nil {
		t.Fatal("expected dependency cycle to be rejected")
	}
	if !strings.Contains(err.Error(), "dependency cycle") {
		t.Fatalf("expected dependency cycle error, got %v", err)
	}
}

func TestParseConfigRejectsInvalidReadyPattern(t *testing.T) {
	content := []byte(`{ "commands": { "core": { "command": "tsc -w", "ready": "[" } } }`)

	_, _, err := parseConfig(content)
	if err == nil {
		t.Fatal("expected invalid ready pattern to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid ready pattern") {
		t.Fatalf("expected invalid ready pattern error, got %v", err)
	}
}

func TestParseConfigRejectsArrayCommands(t *testing.T) {
	content := []byte(`{ "commands": ["npm run dev"] }`)
	if _, _, err := parseConfig(content); err == nil {
		t.Fatal("expected array commands to be rejected")
	}
}

func TestParseConfigReadsGroupAndDefaultGroup(t *testing.T) {
	content := []byte(`{
		"defaultGroup": "services",
		"commands": {
			"api": { "command": "npm run api", "group": "services" },
			"emails": { "command": "npm run build:emails", "group": "build" }
		}
	}`)

	commands, defaultGroup, err := parseConfig(content)
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}
	if defaultGroup != "services" {
		t.Fatalf("expected defaultGroup services, got %q", defaultGroup)
	}
	if commands[0].Group != "services" {
		t.Fatalf("expected api group services, got %q", commands[0].Group)
	}
	if commands[1].Group != "build" {
		t.Fatalf("expected emails group build, got %q", commands[1].Group)
	}
}
