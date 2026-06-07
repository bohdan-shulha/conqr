package main

import "testing"

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
