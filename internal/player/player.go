package player

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// EventType identifies an asynchronous mpv event.
type EventType int

const (
	EventProperty EventType = iota
	EventFileLoaded
	EventEndFile
	EventError
)

const commandTimeout = time.Second

// Event is a state change received from mpv.
type Event struct {
	Type      EventType
	Name      string
	Data      any
	Reason    string
	FileError string
	Err       error
}

type commandResult struct {
	err error
}

type tailBuffer struct {
	mu    sync.Mutex
	data  []byte
	limit int
}

func (b *tailBuffer) Write(content []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, content...)
	if len(b.data) > b.limit {
		b.data = append([]byte(nil), b.data[len(b.data)-b.limit:]...)
	}
	return len(content), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(b.data))
}

type ipcMessage struct {
	Event     string `json:"event"`
	Name      string `json:"name"`
	Data      any    `json:"data"`
	Error     string `json:"error"`
	Reason    string `json:"reason"`
	FileError string `json:"file_error"`
	RequestID uint64 `json:"request_id"`
}

// Player controls one persistent mpv process through JSON IPC.
type Player struct {
	cmd           *exec.Cmd
	conn          net.Conn
	socketPath    string
	socketDir     string
	events        chan Event
	waitCh        chan error
	readDone      chan struct{}
	writeMu       sync.Mutex
	pendingMu     sync.Mutex
	pending       map[uint64]chan commandResult
	nextRequestID uint64
	closeOnce     sync.Once
	stderr        *tailBuffer
}

// Start launches an idle, audio-only mpv process and connects to its IPC socket.
func Start() (*Player, error) {
	mpvPath, err := exec.LookPath("mpv")
	if err != nil {
		return nil, errors.New("mpv PATH içinde bulunamadı")
	}

	socketDir, err := os.MkdirTemp("", "kaset-")
	if err != nil {
		return nil, fmt.Errorf("geçici IPC klasörü oluşturulamadı: %w", err)
	}
	socketPath := filepath.Join(socketDir, "mpv.sock")
	cmd := exec.Command(mpvPath,
		"--no-config",
		"--idle=yes",
		"--terminal=no",
		"--input-terminal=no",
		"--vid=no",
		"--audio-display=no",
		"--volume-max=100",
		"--input-ipc-server="+socketPath,
	)
	// Keep mpv tied to this application even if deferred cleanup cannot run.
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
	stderr := &tailBuffer{limit: 16 * 1024}
	cmd.Stdout = io.Discard
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(socketDir)
		return nil, fmt.Errorf("mpv başlatılamadı: %w", err)
	}

	p := &Player{
		cmd:        cmd,
		socketPath: socketPath,
		socketDir:  socketDir,
		events:     make(chan Event, 64),
		waitCh:     make(chan error, 1),
		readDone:   make(chan struct{}),
		pending:    make(map[uint64]chan commandResult),
		stderr:     stderr,
	}
	go func() {
		p.waitCh <- cmd.Wait()
	}()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr := net.Dial("unix", socketPath)
		if dialErr == nil {
			p.conn = conn
			go p.readLoop()
			if err := p.observeProperties(); err != nil {
				_ = p.Close()
				return nil, fmt.Errorf("mpv durum takibi başlatılamadı: %w", err)
			}
			return p, nil
		}

		select {
		case waitErr := <-p.waitCh:
			_ = os.RemoveAll(socketDir)
			detail := p.stderr.String()
			if waitErr == nil {
				if detail != "" {
					return nil, fmt.Errorf("mpv IPC açılmadan kapandı: %s", detail)
				}
				return nil, errors.New("mpv IPC açılmadan kapandı")
			}
			if detail != "" {
				return nil, fmt.Errorf("mpv IPC açılmadan kapandı: %w: %s", waitErr, detail)
			}
			return nil, fmt.Errorf("mpv IPC açılmadan kapandı: %w", waitErr)
		default:
			time.Sleep(25 * time.Millisecond)
		}
	}

	_ = cmd.Process.Kill()
	<-p.waitCh
	_ = os.RemoveAll(socketDir)
	if detail := stderr.String(); detail != "" {
		return nil, fmt.Errorf("mpv IPC bağlantısı zaman aşımına uğradı: %s", detail)
	}
	return nil, errors.New("mpv IPC bağlantısı zaman aşımına uğradı")
}

// Events returns state changes emitted by mpv.
func (p *Player) Events() <-chan Event {
	return p.events
}

// Load starts playing path and replaces the current mpv playlist.
func (p *Player) Load(path string) error {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("parça yolu çözümlenemedi: %w", err)
	}
	return p.command("loadfile", absolutePath, "replace")
}

// TogglePause toggles playback pause state.
func (p *Player) TogglePause() error {
	return p.command("cycle", "pause")
}

// Stop stops the current track.
func (p *Player) Stop() error {
	return p.command("stop")
}

// Seek moves playback by deltaSeconds.
func (p *Player) Seek(deltaSeconds float64) error {
	return p.command("seek", deltaSeconds, "relative")
}

// ChangeVolume adjusts volume and lets mpv clamp it to its supported range.
func (p *Player) ChangeVolume(delta float64) error {
	return p.command("add", "volume", delta)
}

// SetVolume sets the absolute playback volume.
func (p *Player) SetVolume(volume float64) error {
	return p.command("set_property", "volume", volume)
}

// ToggleMute toggles the mute property.
func (p *Player) ToggleMute() error {
	return p.command("cycle", "mute")
}

func (p *Player) observeProperties() error {
	properties := []string{"time-pos", "duration", "pause", "volume", "mute", "media-title", "metadata"}
	for id, name := range properties {
		if err := p.command("observe_property", id+1, name); err != nil {
			return err
		}
	}
	return nil
}

func (p *Player) command(args ...any) error {
	requestID, result, err := p.sendCommand(args...)
	if err != nil {
		return err
	}

	timer := time.NewTimer(commandTimeout)
	defer timer.Stop()
	select {
	case response := <-result:
		return response.err
	case <-timer.C:
		p.removePending(requestID)
		return errors.New("mpv komutu zaman aşımına uğradı")
	case <-p.readDone:
		p.removePending(requestID)
		return errors.New("mpv bağlantısı kapandı")
	}
}

func (p *Player) sendCommand(args ...any) (uint64, <-chan commandResult, error) {
	p.pendingMu.Lock()
	p.nextRequestID++
	requestID := p.nextRequestID
	result := make(chan commandResult, 1)
	p.pending[requestID] = result
	p.pendingMu.Unlock()

	payload, err := json.Marshal(map[string]any{"command": args, "request_id": requestID})
	if err != nil {
		p.removePending(requestID)
		return 0, nil, fmt.Errorf("mpv komutu kodlanamadı: %w", err)
	}
	payload = append(payload, '\n')

	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	if p.conn == nil {
		p.removePending(requestID)
		return 0, nil, errors.New("mpv bağlantısı kapalı")
	}
	if _, err := p.conn.Write(payload); err != nil {
		p.removePending(requestID)
		return 0, nil, fmt.Errorf("mpv komutu gönderilemedi: %w", err)
	}
	return requestID, result, nil
}

func (p *Player) sendCommandWithoutReply(args ...any) {
	payload, err := json.Marshal(map[string]any{"command": args})
	if err != nil {
		return
	}
	payload = append(payload, '\n')

	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	if p.conn != nil {
		_, _ = p.conn.Write(payload)
	}
}

func (p *Player) removePending(requestID uint64) {
	p.pendingMu.Lock()
	delete(p.pending, requestID)
	p.pendingMu.Unlock()
}

func (p *Player) resolveCommand(requestID uint64, result commandResult) {
	p.pendingMu.Lock()
	waiting, ok := p.pending[requestID]
	if ok {
		delete(p.pending, requestID)
	}
	p.pendingMu.Unlock()
	if ok {
		waiting <- result
	}
}

func (p *Player) failPending(err error) {
	p.pendingMu.Lock()
	pending := p.pending
	p.pending = make(map[uint64]chan commandResult)
	p.pendingMu.Unlock()
	for _, waiting := range pending {
		waiting <- commandResult{err: err}
	}
}

func (p *Player) readLoop() {
	defer close(p.readDone)
	defer close(p.events)
	defer os.RemoveAll(p.socketDir)
	defer p.failPending(errors.New("mpv bağlantısı kapandı"))

	scanner := bufio.NewScanner(p.conn)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	lastPositionEvent := time.Time{}

	for scanner.Scan() {
		message, err := decodeIPCMessage(scanner.Bytes())
		if err != nil {
			p.events <- Event{Type: EventError, Err: err}
			continue
		}
		if message.RequestID != 0 {
			var commandErr error
			if message.Error != "" && message.Error != "success" {
				commandErr = fmt.Errorf("mpv komutu başarısız: %s", message.Error)
			}
			p.resolveCommand(message.RequestID, commandResult{err: commandErr})
			continue
		}

		event, ok := eventFromMessage(message)
		if !ok {
			continue
		}
		if event.Type == EventProperty && event.Name == "time-pos" {
			now := time.Now()
			if event.Data != nil && now.Sub(lastPositionEvent) < 200*time.Millisecond {
				continue
			}
			lastPositionEvent = now
			select {
			case p.events <- event:
			default:
			}
			continue
		}
		p.events <- event
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, net.ErrClosed) {
		p.events <- Event{Type: EventError, Err: fmt.Errorf("mpv bağlantısı kesildi: %w", err)}
	}
}

func decodeIPCMessage(line []byte) (ipcMessage, error) {
	var message ipcMessage
	if err := json.Unmarshal(line, &message); err != nil {
		return ipcMessage{}, fmt.Errorf("mpv yanıtı okunamadı: %w", err)
	}
	return message, nil
}

func eventFromMessage(message ipcMessage) (Event, bool) {
	switch message.Event {
	case "property-change":
		return Event{Type: EventProperty, Name: message.Name, Data: message.Data}, true
	case "file-loaded":
		return Event{Type: EventFileLoaded}, true
	case "end-file":
		return Event{Type: EventEndFile, Reason: message.Reason, FileError: message.FileError}, true
	}
	if message.Error != "" && message.Error != "success" {
		return Event{Type: EventError, Err: fmt.Errorf("mpv komutu başarısız: %s", message.Error)}, true
	}
	return Event{}, false
}

func decodeMessage(line []byte) (Event, bool) {
	message, err := decodeIPCMessage(line)
	if err != nil {
		return Event{Type: EventError, Err: err}, true
	}
	return eventFromMessage(message)
}

// Close asks mpv to quit, then forcefully cleans up if it does not respond.
func (p *Player) Close() error {
	var closeErr error
	p.closeOnce.Do(func() {
		p.sendCommandWithoutReply("quit")

		select {
		case err := <-p.waitCh:
			if err != nil {
				closeErr = err
			}
		case <-time.After(800 * time.Millisecond):
			if p.cmd.Process != nil {
				_ = p.cmd.Process.Kill()
			}
			select {
			case <-p.waitCh:
			case <-time.After(800 * time.Millisecond):
				closeErr = errors.New("mpv süreci sonlandırılamadı")
			}
		}

		if p.conn != nil {
			_ = p.conn.Close()
		}
		select {
		case <-p.readDone:
		case <-time.After(300 * time.Millisecond):
		}
		_ = os.RemoveAll(p.socketDir)
	})
	return closeErr
}
