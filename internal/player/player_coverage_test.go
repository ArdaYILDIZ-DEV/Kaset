//go:build linux

package player

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartFailsWhenMPVNotFound(t *testing.T) {
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	_, err := Start()
	if err == nil || !strings.Contains(err.Error(), "mpv") {
		t.Fatalf("Start() err=%v, want mpv not found", err)
	}
	_ = origPath
}

func TestLoadAbsoluteAndCommandWithoutConnection(t *testing.T) {
	p := &Player{
		pending:  make(map[uint64]chan commandResult),
		events:   make(chan Event, 1),
		readDone: make(chan struct{}),
	}
	// Load should clean path and fail when connection is nil
	if err := p.Load("relative/path.mp3"); err == nil || !strings.Contains(err.Error(), "closed") {
		// original Turkish error contains "bağlantısı kapalı"
		if !strings.Contains(err.Error(), "kapalı") {
			t.Fatalf("Load with nil conn err=%v", err)
		}
	}
	if err := p.TogglePause(); err == nil {
		t.Fatal("TogglePause should fail without connection")
	}
	if err := p.Stop(); err == nil {
		t.Fatal("Stop should fail without connection")
	}
	if err := p.Seek(5); err == nil {
		t.Fatal("Seek should fail without connection")
	}
	if err := p.ChangeVolume(5); err == nil {
		t.Fatal("ChangeVolume should fail without connection")
	}
	if err := p.SetVolume(50); err == nil {
		t.Fatal("SetVolume should fail without connection")
	}
	if err := p.ToggleMute(); err == nil {
		t.Fatal("ToggleMute should fail without connection")
	}
	// Events channel should be readable
	if p.Events() == nil {
		t.Fatal("Events should not be nil")
	}
}

func TestSendCommandAndResolveAndRemovePending(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	p := &Player{
		conn:     client,
		pending:  make(map[uint64]chan commandResult),
		events:   make(chan Event, 8),
		readDone: make(chan struct{}),
	}
	// Test sendCommand directly; command requires readLoop, so use sendCommand + manual resolve
	// net.Pipe is synchronous, so reader must start before writer
	readerDone := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		n, _ := server.Read(buf)
		var req map[string]any
		_ = json.Unmarshal(buf[:n], &req)
		if req["request_id"] == nil {
			t.Error("request_id missing")
		}
		close(readerDone)
	}()
	id, ch, err := p.sendCommand("get_property", "volume")
	if err != nil {
		t.Fatalf("sendCommand err=%v", err)
	}
	if id == 0 || ch == nil {
		t.Fatal("sendCommand should return id and channel")
	}
	<-readerDone
	// simulate successful command resolution
	p.resolveCommand(id, commandResult{err: nil})
	select {
	case res := <-ch:
		if res.err != nil {
			t.Fatalf("resolve err=%v", res.err)
		}
	case <-time.After(time.Second):
		t.Fatal("resolve timeout")
	}
	// simulate error response
	readerDone2 := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		_, _ = server.Read(buf)
		close(readerDone2)
	}()
	id2, ch2, err := p.sendCommand("get_property", "invalid")
	if err != nil {
		t.Fatalf("sendCommand2 err=%v", err)
	}
	<-readerDone2
	p.resolveCommand(id2, commandResult{err: errors.New("mpv command failed: property unavailable")})
	select {
	case res := <-ch2:
		if res.err == nil || !strings.Contains(res.err.Error(), "failed") {
			t.Fatalf("error resolve err=%v", res.err)
		}
	case <-time.After(time.Second):
		t.Fatal("error resolve timeout")
	}
	// removePending
	p.pending[99] = make(chan commandResult, 1)
	p.removePending(99)
	if _, ok := p.pending[99]; ok {
		t.Fatal("removePending did not delete")
	}
	// resolveCommand
	ch3 := make(chan commandResult, 1)
	p.pending[100] = ch3
	p.resolveCommand(100, commandResult{err: errors.New("test")})
	select {
	case res := <-ch3:
		if res.err == nil || res.err.Error() != "test" {
			t.Fatalf("resolveCommand err=%v", res.err)
		}
	case <-time.After(time.Second):
		t.Fatal("resolveCommand timeout")
	}
	// resolving unknown request_id should not panic
	p.resolveCommand(999, commandResult{})
}

func TestFailPendingAndEmitEvent(t *testing.T) {
	p := &Player{
		pending: map[uint64]chan commandResult{
			1: make(chan commandResult, 1),
			2: make(chan commandResult, 1),
		},
		events: make(chan Event, 1),
	}
	p.failPending(errors.New("closed"))
	if len(p.pending) != 0 {
		t.Fatalf("failPending should clear pending: %v", p.pending)
	}
	// emitEvent should not block when channel is full
	p.events <- Event{Type: EventProperty}
	p.emitEvent(Event{Type: EventProperty, Name: "volume"})
	if len(p.events) != 1 {
		t.Fatalf("emitEvent blocked on full channel")
	}
	<-p.events
	p.emitEvent(Event{Type: EventProperty, Name: "volume", Data: 80})
	if len(p.events) != 1 {
		t.Fatal("emitEvent failed on empty channel")
	}
}

func TestDecodeIPCMessageAndEventFromMessage(t *testing.T) {
	// corrupt JSON
	if _, err := decodeIPCMessage([]byte("{ broken")); err == nil {
		t.Fatal("corrupt JSON should fail")
	}
	msg, err := decodeIPCMessage([]byte(`{"request_id":1,"error":"success"}`))
	if err != nil || msg.RequestID != 1 {
		t.Fatalf("decodeIPCMessage err=%v msg=%+v", err, msg)
	}
	// property-change
	evt, ok := eventFromMessage(ipcMessage{Event: "property-change", Name: "volume", Data: 80})
	if !ok || evt.Type != EventProperty || evt.Name != "volume" {
		t.Fatalf("property-change evt=%+v ok=%v", evt, ok)
	}
	// file-loaded
	evt, ok = eventFromMessage(ipcMessage{Event: "file-loaded"})
	if !ok || evt.Type != EventFileLoaded {
		t.Fatalf("file-loaded evt=%+v", evt)
	}
	// end-file
	evt, ok = eventFromMessage(ipcMessage{Event: "end-file", Reason: "eof", FileError: ""})
	if !ok || evt.Type != EventEndFile || evt.Reason != "eof" {
		t.Fatalf("end-file evt=%+v", evt)
	}
	// error
	evt, ok = eventFromMessage(ipcMessage{Error: "some error"})
	if !ok || evt.Type != EventError || evt.Err == nil {
		t.Fatalf("error evt=%+v ok=%v", evt, ok)
	}
	// success should not be treated as error
	_, ok = eventFromMessage(ipcMessage{Error: "success"})
	if ok {
		t.Fatal("success error should not produce event")
	}
	// unknown event
	_, ok = eventFromMessage(ipcMessage{Event: "unknown"})
	if ok {
		t.Fatal("unknown event should not produce event")
	}
}

func TestCommandTimeoutAndReadDone(t *testing.T) {
	// Timeout when server never replies (2s)
	client, server := net.Pipe()
	p := &Player{
		conn:     client,
		pending:  make(map[uint64]chan commandResult),
		events:   make(chan Event, 8),
		readDone: make(chan struct{}),
	}
	go func() {
		buf := make([]byte, 4096)
		_, _ = server.Read(buf)
		time.Sleep(3 * time.Second)
	}()
	start := time.Now()
	err := p.command("get_property", "volume")
	_ = client.Close()
	_ = server.Close()
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		// Turkish message is "zaman aşımı"
		if !strings.Contains(err.Error(), "zaman") && !strings.Contains(err.Error(), "timed") {
			t.Fatalf("timeout err=%v", err)
		}
	}
	if time.Since(start) < 1500*time.Millisecond {
		t.Fatalf("timeout returned too early: %v", time.Since(start))
	}
	// When readDone is closed, should return connection closed
	client2, server2 := net.Pipe()
	p2 := &Player{
		conn:     client2,
		pending:  make(map[uint64]chan commandResult),
		events:   make(chan Event, 8),
		readDone: make(chan struct{}),
	}
	close(p2.readDone)
	go func() {
		buf := make([]byte, 4096)
		_, _ = server2.Read(buf)
		time.Sleep(3 * time.Second)
	}()
	err = p2.command("get_property", "volume")
	_ = client2.Close()
	_ = server2.Close()
	if err == nil || !strings.Contains(err.Error(), "closed") {
		if !strings.Contains(err.Error(), "kapandı") {
			t.Fatalf("readDone err=%v", err)
		}
	}
}

func TestSendCommandWithoutReplyAndClose(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	p := &Player{
		conn:      client,
		pending:   make(map[uint64]chan commandResult),
		events:    make(chan Event, 8),
		readDone:  make(chan struct{}),
		socketDir: t.TempDir(),
		socketPath: filepath.Join(t.TempDir(), "sock"),
	}
	// should not panic with nil conn
	p2 := &Player{}
	p2.sendCommandWithoutReply("quit")
	// normal conn
	go func() {
		buf := make([]byte, 4096)
		n, _ := server.Read(buf)
		if n == 0 {
			t.Error("sendCommandWithoutReply did not write")
		}
	}()
	p.sendCommandWithoutReply("quit")
	time.Sleep(50 * time.Millisecond)

	// Close should be idempotent; simulate process wait
	p3 := &Player{
		pending:   make(map[uint64]chan commandResult),
		events:    make(chan Event, 8),
		readDone:  make(chan struct{}),
		waitCh:    make(chan error, 1),
		socketDir: t.TempDir(),
	}
	close(p3.readDone)
	p3.waitCh <- nil
	if err := p3.Close(); err != nil {
		t.Fatalf("Close err=%v", err)
	}
	if err := p3.Close(); err != nil {
		t.Fatalf("second Close err=%v", err)
	}
}

func TestTailBufferAndNumberOrZero(t *testing.T) {
	b := &tailBuffer{limit: 10}
	if _, err := b.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); got != "hello" {
		t.Fatalf("String %q", got)
	}
	if _, err := b.Write([]byte("0123456789ABC")); err != nil {
		t.Fatal(err)
	}
	if len(b.String()) > 10 {
		t.Fatalf("limit exceeded %q", b.String())
	}
	empty := &tailBuffer{limit: 5}
	if got := empty.String(); got != "" {
		t.Fatalf("empty String %q", got)
	}
}

func TestReadLoopHandlesPropertyAndEndFile(t *testing.T) {
	client, server := net.Pipe()
	p := &Player{
		conn:     client,
		pending:  make(map[uint64]chan commandResult),
		events:   make(chan Event, 8),
		readDone: make(chan struct{}),
		waitCh:   make(chan error, 1),
		socketDir: t.TempDir(),
	}
	go p.readLoop()
	msg := `{"event":"property-change","name":"volume","data":80}` + "\n"
	if _, err := server.Write([]byte(msg)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	select {
	case evt := <-p.events:
		if evt.Type != EventProperty || evt.Name != "volume" {
			t.Fatalf("readLoop property evt=%+v", evt)
		}
	case <-time.After(time.Second):
		t.Fatal("property event not received")
	}
	msg2 := `{"event":"end-file","reason":"eof"}` + "\n"
	if _, err := server.Write([]byte(msg2)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	select {
	case evt := <-p.events:
		if evt.Type != EventEndFile || evt.Reason != "eof" {
			t.Fatalf("end-file evt=%+v", evt)
		}
	case <-time.After(time.Second):
		t.Fatal("end-file event not received")
	}
	// corrupt line should produce error event
	if _, err := server.Write([]byte("{ broken\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	select {
	case evt := <-p.events:
		if evt.Type != EventError {
			t.Fatalf("corrupt line should produce EventError %+v", evt)
		}
	case <-time.After(time.Second):
		t.Fatal("corrupt line error event not received")
	}
	_ = client.Close()
	_ = server.Close()
	<-p.readDone
}
