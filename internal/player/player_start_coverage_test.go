//go:build linux

package player

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// fakeMPVSource is a minimal mpv replacement that creates the IPC socket
// and replies with success to every request_id. It is compiled on demand
// into a temp directory and placed on PATH as "mpv".
const fakeMPVSource = `package main

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"strings"
)

func main() {
	var socket string
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "--input-ipc-server=") {
			socket = strings.TrimPrefix(arg, "--input-ipc-server=")
		}
	}
	if socket == "" {
		os.Exit(1)
	}
	_ = os.Remove(socket)
	ln, err := net.Listen("unix", socket)
	if err != nil {
		os.Exit(1)
	}
	defer ln.Close()
	defer os.Remove(socket)
	conn, err := ln.Accept()
	if err != nil {
		os.Exit(1)
	}
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var req map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}
		if id, ok := req["request_id"]; ok {
			resp := map[string]any{"request_id": id, "error": "success"}
			data, _ := json.Marshal(resp)
			_, _ = conn.Write(append(data, '\n'))
		}
	}
}
`

func buildFakeMPV(t *testing.T, dir string) string {
	t.Helper()
	src := filepath.Join(dir, "fake_mpv.go")
	if err := os.WriteFile(src, []byte(fakeMPVSource), 0o600); err != nil {
		t.Fatalf("write fake source: %v", err)
	}
	bin := filepath.Join(dir, "mpv")
	cmd := exec.Command("go", "build", "-o", bin, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake mpv: %v %s", err, out)
	}
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatalf("chmod fake mpv: %v", err)
	}
	return bin
}

func TestStartWithFakeMPVSuccess(t *testing.T) {
	dir := t.TempDir()
	buildFakeMPV(t, dir)
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+":"+origPath)
	p, err := Start()
	if err != nil {
		t.Fatalf("Start with fake mpv failed: %v", err)
	}
	defer func() { _ = p.Close() }()
	// Events channel should be open
	if p.Events() == nil {
		t.Fatal("Events should not be nil after Start")
	}
	// Load should succeed via fake (it will be sent and replied)
	tmp := filepath.Join(dir, "test.mp3")
	if err := os.WriteFile(tmp, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = p.Load(tmp)
	// Close should not error
	if err := p.Close(); err != nil && !isNormalCloseError(err) {
		t.Fatalf("Close after fake mpv err=%v", err)
	}
	// second Close should be idempotent
	if err := p.Close(); err != nil && !isNormalCloseError(err) {
		t.Fatalf("second Close err=%v", err)
	}
}

func isNormalCloseError(err error) bool {
	// Close may return nil or process error; both are acceptable
	return err == nil
}

func TestStartFailsWhenMPVExitsImmediately(t *testing.T) {
	dir := t.TempDir()
	// fake that exits 1 immediately
	script := filepath.Join(dir, "mpv")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+":"+origPath)
	_, err := Start()
	if err == nil {
		t.Fatal("Start with exiting mpv should fail")
	}
}

func TestStartFailsWhenMPVSlowToCreateSocket(t *testing.T) {
	dir := t.TempDir()
	// fake that sleeps longer than Start deadline (3s) before creating socket
	script := filepath.Join(dir, "mpv")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nsleep 5\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+":"+origPath)
	start := time.Now()
	_, err := Start()
	if err == nil {
		t.Fatal("Start with slow mpv should timeout")
	}
	if time.Since(start) < 2*time.Second {
		t.Fatalf("timeout too early %v", time.Since(start))
	}
}

func TestObservePropertiesDirectly(t *testing.T) {
	dir := t.TempDir()
	buildFakeMPV(t, dir)
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+":"+origPath)
	p, err := Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer p.Close()
	// observeProperties is called during Start, but calling again should succeed
	if err := p.observeProperties(); err != nil {
		t.Fatalf("observeProperties second call err=%v", err)
	}
}
