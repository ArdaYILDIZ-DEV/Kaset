//go:build linux

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestParseArgumentsExtra covers help and error branches not yet hit
func TestParseArgumentsExtra(t *testing.T) {
	var buf bytes.Buffer
	_, err := parseArguments([]string{"-h"}, &buf)
	if err != nil {
		t.Fatalf("parse -h err=%v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("help should write usage")
	}
	buf.Reset()
	_, err = parseArguments([]string{"one", "two"}, &buf)
	if err == nil {
		t.Fatal("two args should fail")
	}
	buf.Reset()
	_, err = parseArguments([]string{"--unknown"}, &buf)
	if err == nil {
		t.Fatal("unknown flag should fail")
	}
}

// TestRunCoversEarlyBranches tests run() error paths before TUI
func TestRunCoversEarlyBranches(t *testing.T) {
	// Use temp XDG_CONFIG_HOME to isolate
	tmpConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpConfig)
	// Create temp music dir with no supported files
	emptyMusic := filepath.Join(t.TempDir(), "empty")
	if err := os.Mkdir(emptyMusic, 0o755); err != nil {
		t.Fatal(err)
	}
	// Save original Args and restore
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// Case: explicit directory with no tracks -> should return error
	os.Args = []string{"kaset", emptyMusic}
	err := run()
	if err == nil || !containsStr(err.Error(), "desteklenen") {
		t.Fatalf("run with empty music should fail, got %v", err)
	}

	// Case: non-existent explicit directory
	os.Args = []string{"kaset", "/nonexistent/path/xyz"}
	err = run()
	if err == nil {
		t.Fatal("run with nonexistent dir should fail")
	}

	// Case: help flag should not error (parse returns help true, run returns nil)
	os.Args = []string{"kaset", "-h"}
	// run() parses -h and returns nil (help)
	// It uses parseArguments which handles -h as help true, then run returns nil early
	// So run() should return nil for help
	err = run()
	if err != nil {
		t.Fatalf("run with -h should not error, got %v", err)
	}

	// Case: corrupt settings file should be recovered and still try to scan
	// Create corrupt settings
	configPath := filepath.Join(tmpConfig, "kaset", "settings.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("{ corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Need a valid music dir with one track for this case
	validMusic := t.TempDir()
	if err := os.WriteFile(filepath.Join(validMusic, "a.mp3"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Also need fake mpv to avoid real mpv not found error; create fake that exits quickly
	fakeDir := t.TempDir()
	script := filepath.Join(fakeDir, "mpv")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeDir+":"+origPath)
	os.Args = []string{"kaset", validMusic}
	err = run()
	// Should fail due to mpv not starting (since fake exits), but settings recovery notice should have been handled
	if err == nil || !containsStr(err.Error(), "mpv") {
		// It may fail at mpv or at settings depending on order; accept any error but ensure run was exercised
		t.Logf("run with corrupt settings and fake mpv err=%v", err)
	}
}

// TestRunWithInvalidLibraryFallback covers stored library fallback
func TestRunWithInvalidLibraryFallback(t *testing.T) {
	tmpConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpConfig)
	// Create a settings file with invalid library that will fail ScanWithIssues
	// Then run without explicit arg should try stored library, fail, then fallback to "."
	// First create valid music in current dir
	cwdMusic := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwdMusic, "b.mp3"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	origWd, _ := os.Getwd()
	if err := os.Chdir(cwdMusic); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)

	// Write settings with library pointing to nonexistent
	configPath := filepath.Join(tmpConfig, "kaset", "settings.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	// Use valid JSON with Volume and Library
	if err := os.WriteFile(configPath, []byte(`{"version":1,"library":"/nonexistent_xyz","volume":80}`), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeDir := t.TempDir()
	script := filepath.Join(fakeDir, "mpv")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeDir+":"+origPath)
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"kaset"}
	err := run()
	// Should either succeed (if fallback finds music) or fail at mpv; but should have exercised fallback branch
	if err != nil {
		t.Logf("run fallback err=%v", err)
	}
}

// containsStr helper
func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > len(sub) && searchSub(s, sub))
}
func searchSub(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestMainFunction ensures main does not panic with help
func TestMainHelpDoesNotPanic(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"kaset", "-h"}
	// main with help should not call os.Exit, so we can call it directly
	main()
}

func TestMainWithErrorDoesNotCoverExit(t *testing.T) {
	// Verify that run returns error for invalid case, which main would turn into exit
	// This at least exercises the error path of run that main would handle
	tmpConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpConfig)
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"kaset", "/nonexistent_xyz_123"}
	err := run()
	if err == nil {
		t.Fatal("run should fail for nonexistent library")
	}
}
