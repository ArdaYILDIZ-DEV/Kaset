package player

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func TestTailBufferKeepsBoundedSuffix(t *testing.T) {
	buffer := &tailBuffer{limit: 5}
	if _, err := buffer.Write([]byte("1234")); err != nil {
		t.Fatal(err)
	}
	if _, err := buffer.Write([]byte("567")); err != nil {
		t.Fatal(err)
	}
	if got := buffer.String(); got != "34567" {
		t.Fatalf("String() = %q", got)
	}
}

func TestDecodePropertyChange(t *testing.T) {
	event, ok := decodeEvent(t, `{"event":"property-change","name":"volume","data":42.5}`)
	if !ok {
		t.Fatal("eventFromMessage() ok = false")
	}
	if event.Type != EventProperty || event.Name != "volume" || event.Data != 42.5 {
		t.Fatalf("eventFromMessage() = %#v", event)
	}
}

func TestDecodeEndFile(t *testing.T) {
	event, ok := decodeEvent(t, `{"event":"end-file","reason":"error","file_error":"unrecognized file format"}`)
	if !ok || event.Type != EventEndFile || event.Reason != "error" || event.FileError != "unrecognized file format" {
		t.Fatalf("eventFromMessage() = %#v, %v", event, ok)
	}
}

func TestDecodeCommandResponse(t *testing.T) {
	message, err := decodeIPCMessage([]byte(`{"request_id":12,"error":"success"}`))
	if err != nil {
		t.Fatal(err)
	}
	if message.RequestID != 12 || message.Error != "success" {
		t.Fatalf("decodeIPCMessage() = %#v", message)
	}
}

func TestDecodeCommandError(t *testing.T) {
	event, ok := decodeEvent(t, `{"error":"property unavailable"}`)
	if !ok || event.Type != EventError || event.Err == nil {
		t.Fatalf("eventFromMessage() = %#v, %v", event, ok)
	}
}

func decodeEvent(t *testing.T, line string) (Event, bool) {
	t.Helper()
	message, err := decodeIPCMessage([]byte(line))
	if err != nil {
		t.Fatal(err)
	}
	return eventFromMessage(message)
}

func TestPlayerIntegration(t *testing.T) {
	if os.Getenv("KASET_INTEGRATION") != "1" {
		t.Skip("set KASET_INTEGRATION=1 to run with mpv")
	}

	mpv, err := Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mpv.Close() })
	if err := mpv.SetVolume(37); err != nil {
		t.Fatalf("SetVolume() error = %v", err)
	}

	track := writeSilentWAV(t)
	if err := mpv.Load(track); err != nil {
		_ = mpv.Close()
		t.Fatal(err)
	}

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	loaded := false
	for !loaded {
		select {
		case event, ok := <-mpv.Events():
			if !ok {
				t.Fatal("mpv event channel closed before file-loaded")
			}
			if event.Type == EventError {
				t.Fatalf("mpv event error: %v", event.Err)
			}
			loaded = event.Type == EventFileLoaded
		case <-timer.C:
			t.Fatal("timed out waiting for mpv file-loaded event")
		}
	}

	pid := mpv.cmd.Process.Pid
	if err := mpv.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !waitForProcessExit(pid, 2*time.Second) {
		t.Fatalf("mpv process %d survived Close()", pid)
	}
}

type parentDeathRecord struct {
	PID       int    `json:"pid"`
	SocketDir string `json:"socket_dir"`
}

func TestParentDeathStopsMPV(t *testing.T) {
	if os.Getenv("KASET_INTEGRATION") != "1" {
		t.Skip("set KASET_INTEGRATION=1 to run with mpv")
	}
	if runtime.GOOS != "linux" {
		t.Skip("parent-death signal is Linux-specific")
	}

	recordPath := filepath.Join(t.TempDir(), "player.json")
	command := exec.Command(os.Args[0], "-test.run=^TestParentDeathHelper$")
	command.Env = append(os.Environ(),
		"KASET_PARENT_DEATH_HELPER=1",
		"KASET_PARENT_DEATH_RECORD="+recordPath,
	)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("parent-death helper failed: %v\n%s", err, output)
	}

	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	var record parentDeathRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(record.SocketDir) })
	if !waitForProcessExit(record.PID, 5*time.Second) {
		t.Fatalf("mpv process %d survived its parent", record.PID)
	}
}

func TestParentDeathHelper(t *testing.T) {
	if os.Getenv("KASET_PARENT_DEATH_HELPER") != "1" {
		t.Skip("helper process only")
	}

	mpv, err := Start()
	if err != nil {
		t.Fatal(err)
	}
	record := parentDeathRecord{PID: mpv.cmd.Process.Pid, SocketDir: mpv.socketDir}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("KASET_PARENT_DEATH_RECORD"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	// Simulate an abrupt terminal/process closure: deferred Player.Close cannot run.
	os.Exit(0)
}

func writeSilentWAV(t *testing.T) string {
	t.Helper()
	const (
		sampleRate    = 8000
		bitsPerSample = 16
		sampleCount   = 2000
	)
	dataSize := sampleCount * (bitsPerSample / 8)
	buffer := bytes.NewBuffer(make([]byte, 0, 44+dataSize))
	buffer.WriteString("RIFF")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(36+dataSize))
	buffer.WriteString("WAVEfmt ")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(16))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(1))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(1))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(buffer, binary.LittleEndian, uint32(sampleRate*bitsPerSample/8))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(bitsPerSample/8))
	_ = binary.Write(buffer, binary.LittleEndian, uint16(bitsPerSample))
	buffer.WriteString("data")
	_ = binary.Write(buffer, binary.LittleEndian, uint32(dataSize))
	buffer.Write(make([]byte, dataSize))

	path := filepath.Join(t.TempDir(), "silent.wav")
	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitForProcessExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}
