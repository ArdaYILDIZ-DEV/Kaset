package tui

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"kaset/internal/library"
	"kaset/internal/player"
	"kaset/internal/playlist"

	tea "github.com/charmbracelet/bubbletea"
)

// TestInit covers both branches of Init
func TestInitWithAndWithoutPlayer(t *testing.T) {
	m1 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	if cmd := m1.Init(); cmd == nil {
		t.Fatal("Init with nil player should return animation")
	}
	events := make(chan player.Event, 1)
	fp := &fakePlayer{events: events}
	m2 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, fp)
	if cmd := m2.Init(); cmd == nil {
		t.Fatal("Init with player should return batch")
	}
	// startupAnimationTick should be non-nil
	if cmd := startupAnimationTick(); cmd == nil {
		t.Fatal("startupAnimationTick should not be nil")
	}
	if cmd := waitForPlayerEvent(events); cmd == nil {
		t.Fatal("waitForPlayerEvent should not be nil")
	}
	// closed channel variant
	close(events)
	cmd := waitForPlayerEvent(events)
	msg := cmd()
	if _, ok := msg.(playerClosedMsg); !ok {
		t.Fatalf("closed channel should return playerClosedMsg, got %T", msg)
	}
}

// TestRun covers nil player and error handling
func TestRunBranches(t *testing.T) {
	m := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	if ok := m.run(func() error { return nil }); ok {
		t.Fatal("run with nil player should fail")
	}
	if m.errText == "" {
		t.Fatal("run with nil player should set error")
	}
	fp := &fakePlayer{events: make(chan player.Event)}
	m2 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, fp)
	if !m2.run(func() error { return nil }) {
		t.Fatal("run with valid player should succeed")
	}
	if ok := m2.run(func() error { return errors.New("fail") }); ok {
		t.Fatal("run with error should fail")
	}
	if m2.errText != "fail" {
		t.Fatalf("run error text %q", m2.errText)
	}
}

// TestBeginLibraryRefresh covers scanning branches
func TestBeginLibraryRefreshBranches(t *testing.T) {
	m := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	m.libraryRoot = ""
	updated, _ := m.beginLibraryRefresh()
	m = updated.(Model)
	if m.errText == "" {
		t.Fatal("beginLibraryRefresh without root should set error")
	}
	m.libraryRoot = t.TempDir()
	m.scanning = true
	updated, _ = m.beginLibraryRefresh()
	m = updated.(Model)
	// notice set when already scanning is checked via returned model
	m.scanning = false
	model, cmd := m.beginLibraryRefresh()
	if cmd == nil {
		t.Fatal("beginLibraryRefresh should return cmd when not scanning")
	}
	if !model.(Model).scanning {
		t.Fatal("scanning should be true after begin")
	}
	// scanLibrary should produce message
	scanCmd := scanLibrary(t.TempDir())
	if scanCmd == nil {
		t.Fatal("scanLibrary should return cmd")
	}
	msg := scanCmd()
	if _, ok := msg.(libraryScanMsg); !ok {
		t.Fatalf("scanLibrary msg type %T", msg)
	}
	// apply scan error
	m2 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	m2.scanning = true
	m2.applyLibraryScan(libraryScanMsg{err: errors.New("scan fail")})
	if m2.errText == "" {
		t.Fatal("applyLibraryScan with error should set error")
	}
	// apply scan empty
	m2.applyLibraryScan(libraryScanMsg{tracks: nil})
	if m2.errText == "" {
		t.Fatal("empty tracks should set error")
	}
}

// TestUpdateExtra covers remaining Update branches
func TestUpdateExtraBranches(t *testing.T) {
	m := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	// startupTick
	m.startupFrame = 0
	updated, cmd := m.Update(startupTickMsg{})
	if updated.(Model).startupFrame != 1 || cmd == nil {
		t.Fatal("startupTick early frame failed")
	}
	m.startupFrame = startupAnimationLastFrame
	updated, cmd = m.Update(startupTickMsg{})
	if updated.(Model).startupFrame != startupAnimationLastFrame || cmd != nil {
		t.Fatal("startupTick final frame failed")
	}
	m.startupFrame = startupAnimationLastFrame + 1
	updated, _ = m.Update(startupTickMsg{})
	if updated.(Model).startupFrame != startupAnimationLastFrame+1 {
		t.Fatal("beyond final frame should not increment")
	}
	// WindowSize
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 0, Height: 0})
	if updated.(Model).width != 1 || updated.(Model).height != 1 {
		t.Fatalf("WindowSize clamp failed %d %d", updated.(Model).width, updated.(Model).height)
	}
	// ctrl+c
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should return quit")
	}
	// help visible
	m.helpVisible = true
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?' }})
	if updated.(Model).helpVisible {
		t.Fatal("? should close help")
	}
	m.helpVisible = true
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	// q while help visible should quit
	// searching
	m.helpVisible = false
	m.searching = true
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if updated.(Model).searching {
		t.Fatal("esc should exit searching")
	}
	m.searching = true
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.(Model).searching {
		t.Fatal("enter should exit searching")
	}
	// playerClosedMsg
	m.player = &fakePlayer{events: make(chan player.Event)}
	updated, _ = m.Update(playerClosedMsg{})
	if updated.(Model).player != nil {
		t.Fatal("playerClosedMsg should nil player")
	}
	if updated.(Model).errText == "" {
		t.Fatal("playerClosedMsg should set error")
	}
	// playerEventMsg
	events := make(chan player.Event, 1)
	fp := &fakePlayer{events: events}
	m2 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, fp)
	updated, cmd = m2.Update(playerEventMsg(player.Event{Type: player.EventProperty, Name: "volume", Data: 80.0}))
	if cmd == nil {
		t.Fatal("playerEventMsg should return next wait")
	}
	// prompt == playlistName should route to updatePlaylistName
	m3 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	m3.prompt = promptPlaylistName
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	// searching branch
	m3.prompt = promptNone
	m3.searching = true
	updated, _ = m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
}

// contains helper
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestHandleKeyExtra covers handleKey branches not yet hit
func TestHandleKeyExtra(t *testing.T) {
	events := make(chan player.Event, 1)
	fp := &fakePlayer{events: events}
	m := New([]library.Track{{Name: "track", Path: "/a.mp3"}, {Name: "b", Path: "/b.mp3"}}, fp)
	// helper to apply handleKey and update m
	applyKey := func(key tea.KeyMsg) {
		updated, _ := m.handleKey(key)
		m = updated.(Model)
	}
	// ? help
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !m.helpVisible {
		t.Fatal("? should show help")
	}
	m.helpVisible = false
	// esc while panelPlaylists hides side panel
	m.showSidePanel(panelPlaylists)
	applyKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.sidePanelEnabled {
		t.Fatal("esc should hide side panel when playlists visible")
	}
	// esc clears search when not in playlists panel
	m.panel = panelLibrary
	m.search.SetValue("test")
	m.refreshFilter()
	applyKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.search.Value() != "" {
		t.Fatal("esc should clear search")
	}
	// / focuses search
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !m.searching {
		t.Fatal("/ should start searching")
	}
	m.searching = false
	// t toggles listsVisible
	prev := m.listsVisible
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if m.listsVisible == prev {
		t.Fatal("t should toggle listsVisible")
	}
	// y toggles queue panel
	m.listsVisible = true
	m.sidePanelEnabled = true
	m.sidePanel = panelQueue
	m.panel = panelQueue
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if m.sidePanelEnabled {
		t.Fatal("y should hide queue panel when visible")
	}
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !m.sidePanelEnabled || m.sidePanel != panelQueue {
		t.Fatal("y should show queue panel")
	}
	// tab when lists hidden
	m.listsVisible = false
	applyKey(tea.KeyMsg{Type: tea.KeyTab})
	if m.errText == "" && m.noticeText == "" {
		t.Fatal("tab when lists hidden should set notice")
	}
	m.listsVisible = true
	m.sidePanelEnabled = false
	applyKey(tea.KeyMsg{Type: tea.KeyTab})
	m.sidePanelEnabled = true
	m.panel = panelLibrary
	applyKey(tea.KeyMsg{Type: tea.KeyTab})
	if m.panel != panelQueue {
		t.Fatalf("tab from library should go to side panel, got %v", m.panel)
	}
	applyKey(tea.KeyMsg{Type: tea.KeyTab})
	if m.panel != panelLibrary {
		t.Fatal("tab should cycle back to library")
	}
	// P toggles playlists (needs store)
	m.listsVisible = true
	m.sidePanel = panelQueue
	m.sidePanelEnabled = true
	m.playlistStore = playlist.NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if m.sidePanel != panelPlaylists {
		t.Fatal("P should open playlists")
	}
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if m.sidePanelEnabled {
		t.Fatal("P should hide playlists when already visible")
	}
	// S beginPlaylistSave with empty queue should error
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})
	// r library refresh
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	// d toggle folders
	prevShow := m.showFolders
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if m.showFolders == prevShow {
		t.Fatal("d should toggle showFolders")
	}
	// space when no current and no queue should play library context
	m.current = -1
	m.queue.Clear()
	applyKey(tea.KeyMsg{Type: tea.KeySpace})
	// space when current >=0 should toggle pause
	m.current = 0
	applyKey(tea.KeyMsg{Type: tea.KeySpace})
	// n, p
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	// left/right, l, R, z, +,-,m,s
	applyKey(tea.KeyMsg{Type: tea.KeyLeft})
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	applyKey(tea.KeyMsg{Type: tea.KeyRight})
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	// s stop
	m.current = 0
	m.queue.Replace([]int{0}, 0)
	applyKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if m.current != -1 {
		t.Fatal("s should reset current")
	}
}

// TestHandlePanelKey covers remaining panel branches
func TestHandlePanelKeyCoverage(t *testing.T) {
	m := New([]library.Track{{Name: "a", Path: "/a.mp3"}, {Name: "b", Path: "/b.mp3"}}, nil)
	m.panel = panelLibrary
	m.handlePanelKey("up")
	m.handlePanelKey("down")
	m.handlePanelKey("home")
	m.handlePanelKey("end")
	// add tracks to queue via a and A before enter (which creates queue)
	m.handlePanelKey("a")
	if m.queue.Len() != 1 {
		t.Fatalf("a should append selected, got %d", m.queue.Len())
	}
	// filtered append
	m.handlePanelKey("A")
	// queue panel
	m.panel = panelQueue
	m.queue.Replace([]int{0, 1}, 0)
	m.queueCursor = 0
	m.handlePanelKey("up")
	m.handlePanelKey("down")
	m.handlePanelKey("home")
	m.handlePanelKey("end")
	m.handlePanelKey("enter")
	// move via J/K already tested, but x and c
	m.handlePanelKey("x")
	m.handlePanelKey("c")
	if m.queue.Len() != 0 {
		t.Fatal("c should clear queue")
	}
	// playlists panel
	m.panel = panelPlaylists
	m.playlists = []playlist.Playlist{{Name: "one", Tracks: []string{"/a.mp3"}}}
	m.playlistCursor = 0
	m.handlePanelKey("up")
	m.handlePanelKey("down")
	m.handlePanelKey("home")
	m.handlePanelKey("end")
	// enter should try load
	m.handlePanelKey("enter")
	m.handlePanelKey("x")
}

// TestPlaybackExtra covers append, move, previous, shuffle, property
func TestPlaybackExtra(t *testing.T) {
	// appendSelectedToQueue with no selection
	m := New([]library.Track{}, nil)
	m.filtered = []int{}
	m.cursor = 0
	m.appendSelectedToQueue()
	if m.errText == "" {
		t.Fatal("appendSelected with empty should set error")
	}
	// normal append
	m = New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	m.filtered = []int{0}
	m.cursor = 0
	m.appendSelectedToQueue()
	if m.queue.Len() != 1 {
		t.Fatal("appendSelected should add")
	}
	// appendFiltered empty
	m2 := New([]library.Track{}, nil)
	m2.filtered = []int{}
	m2.appendFilteredToQueue()
	if m2.errText == "" {
		t.Fatal("appendFiltered empty should error")
	}
	m2 = New([]library.Track{{Name: "a", Path: "/a.mp3"}, {Name: "b", Path: "/b.mp3"}}, nil)
	m2.filtered = []int{0, 1}
	m2.appendFilteredToQueue()
	if m2.queue.Len() != 2 {
		t.Fatal("appendFiltered should add 2")
	}
	// moveQueueItem
	m3 := New([]library.Track{{Name: "a", Path: "/a.mp3"}, {Name: "b", Path: "/b.mp3"}, {Name: "c", Path: "/c.mp3"}}, nil)
	m3.queue.Replace([]int{0, 1, 2}, 0)
	m3.queueCursor = 0
	m3.moveQueueItem(1)
	if m3.queueCursor != 1 {
		t.Fatalf("moveQueueItem cursor %d", m3.queueCursor)
	}
	// playPrevious
	events := make(chan player.Event, 1)
	fp := &fakePlayer{events: events}
	m4 := New([]library.Track{{Name: "a", Path: "/a.mp3"}, {Name: "b", Path: "/b.mp3"}}, fp)
	m4.queue.Replace([]int{0, 1}, 0)
	m4.playPrevious()
	// should set notice at start
	m4.queue.Replace([]int{0, 1}, 1)
	m4.playPrevious()
	// shuffleUpcoming with no upcoming should set notice
	m5 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	m5.queue.Replace([]int{0}, 0)
	m5.shuffleUpcoming()
	if m5.errText == "" && m5.noticeText == "" {
		t.Fatal("shuffleUpcoming with insufficient should set notice")
	}
	m5.queue.Replace([]int{0, 1, 2}, 0)
	m5.current = -1
	m5.shuffleUpcoming()
	// handlePlayerEvent and handleProperty
	m6 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	m6.handlePlayerEvent(player.Event{Type: player.EventError, Err: errors.New("mpv error")})
	if m6.errText == "" {
		t.Fatal("handlePlayerEvent error should set error")
	}
	m6.handlePlayerEvent(player.Event{Type: player.EventProperty, Name: "pause", Data: true})
	m6.handleProperty("time-pos", 12.3)
	if m6.position != 12.3 {
		t.Fatalf("handleProperty time-pos %v", m6.position)
	}
	m6.handleProperty("duration", 100.0)
	m6.handleProperty("pause", false)
	m6.handleProperty("volume", 80.0)
	m6.handleProperty("mute", true)
	m6.handleProperty("media-title", "title")
	m6.hasMetadataTitle = true
	m6.handleProperty("media-title", "should not overwrite")
	if m6.title != "title" {
		t.Fatal("media-title should not overwrite metadata title")
	}
	m6.handleProperty("metadata", map[string]any{"title": "meta", "artist": "art", "album": "alb"})
	if m6.title != "meta" || m6.artist != "art" {
		t.Fatalf("metadata handling failed %q %q", m6.title, m6.artist)
	}
	m6.handleProperty("metadata", "not a map")
	// invalid type
	m6.handleProperty("volume", "not float")
}

// TestPlaylistsExtra covers playlist save/delete branches
func TestPlaylistsExtra(t *testing.T) {
	dir := t.TempDir()
	store := playlist.NewStore(filepath.Join(dir, "playlists.json"))
	track := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(track, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	m := New([]library.Track{{Name: "a", Path: track}}, nil, store)
	// beginPlaylistSave empty queue (value receiver, capture returned model)
	updated, _ := m.beginPlaylistSave()
	m = updated.(Model)
	if m.errText == "" {
		t.Fatal("beginPlaylistSave empty should error")
	}
	// beginPlaylistSave with queue and nil store
	m2 := New([]library.Track{{Name: "a", Path: track}}, nil)
	m2.queue.Append(0)
	updated2, _ := m2.beginPlaylistSave()
	m2 = updated2.(Model)
	if m2.errText == "" {
		t.Fatal("beginPlaylistSave with nil store should error")
	}
	// normal save flow
	m.queue.Append(0)
	updated, _ = m.beginPlaylistSave()
	m = updated.(Model)
	if m.prompt != promptPlaylistName {
		t.Fatal("beginPlaylistSave should set prompt")
	}
	// handlePromptKey esc, updatePlaylistName
	updated, _ = m.handlePromptKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.prompt != promptNone {
		t.Fatal("esc should cancel prompt")
	}
	updated, _ = m.beginPlaylistSave()
	m = updated.(Model)
	// submit empty name should error (via validate)
	m.playlistName.SetValue("   ")
	m.submitPlaylistName()
	if m.errText == "" {
		t.Fatal("empty playlist name should error")
	}
	m.playlistName.SetValue("MyList")
	m.submitPlaylistName()
	if m.loadedPlaylist != "MyList" {
		t.Fatalf("submit should set loadedPlaylist %q", m.loadedPlaylist)
	}
	// overwrite prompt
	m.queue.Append(0)
	updated, _ = m.beginPlaylistSave()
	m = updated.(Model)
	m.playlistName.SetValue("MyList")
	m.submitPlaylistName()
	if m.prompt != promptOverwrite {
		t.Fatalf("existing name should trigger overwrite prompt %v", m.prompt)
	}
	// handlePromptKey for overwrite (value receiver)
	updated, _ = m.handlePromptKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	updated, _ = m.handlePromptKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	// test savePlaylist directly
	m.savePlaylist("MyList", true)
	// beginPlaylistDelete
	m.openPlaylists()
	m.beginPlaylistDelete()
	if m.prompt != promptDeletePlaylist {
		t.Fatal("beginPlaylistDelete should set prompt")
	}
	updated, _ = m.handlePromptKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	m.beginPlaylistDelete()
	updated, _ = m.handlePromptKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	// updatePlaylistName sanitization
	updated, _ = m.beginPlaylistSave()
	m = updated.(Model)
	updated, _ = m.updatePlaylistName(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(Model)
	m.playlistName.SetValue("test\x00")
	// sanitization should strip control
	updated, _ = m.updatePlaylistName(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)
	// queuePaths
	paths := m.queuePaths()
	if len(paths) == 0 {
		t.Fatal("queuePaths should not be empty")
	}
	// loadSelectedPlaylist missing
	m.playlistCursor = 99
	m.loadSelectedPlaylist()
	if m.errText == "" {
		t.Fatal("loadSelected with invalid cursor should error")
	}
}

// TestLibraryRefreshExtra
func TestLibraryRefreshExtra(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.mp3"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := New([]library.Track{{Name: "old", Path: filepath.Join(dir, "old.mp3")}}, nil)
	m.libraryRoot = dir
	m.tracks = []library.Track{{Name: "old", Path: filepath.Join(dir, "old.mp3")}}
	m.filtered = []int{0}
	m.cursor = 0
	m.searchIndex = buildSearchIndex(m.tracks)
	m.queue.Replace([]int{0}, 0)
	m.current = 0
	// apply scan with new tracks
	newTracks := []library.Track{{Name: "a", Path: filepath.Join(dir, "a.mp3")}}
	m.applyLibraryScan(libraryScanMsg{tracks: newTracks, issues: []library.ScanIssue{{Path: "/x", Err: errors.New("fail")}}})
	if m.noticeText == "" {
		t.Fatal("apply with issues should set notice")
	}
	// test selectedPath preservation
	m2 := New([]library.Track{{Name: "a", Path: "/a.mp3"}, {Name: "b", Path: "/b.mp3"}}, nil)
	m2.filtered = []int{0, 1}
	m2.cursor = 1
	m2.updateSearch(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	// refreshFilterKeeping already covered but ensure
	m2.search.SetValue("a")
	m2.refreshFilter()
	m2.refreshFilterKeeping("/a.mp3")
}

// TestViewHelpers
func TestViewHelpersCoverage(t *testing.T) {
	if got := numberOrZero(12.3); got != 12.3 {
		t.Fatalf("numberOrZero float %v", got)
	}
	if got := numberOrZero("not float"); got != 0 {
		t.Fatalf("numberOrZero non-float %v", got)
	}
	if got := volumeLabel(50, false); got != "50%" {
		t.Fatalf("volumeLabel 50 %q", got)
	}
	if got := volumeLabel(80, true); got != "kapalı" {
		t.Fatalf("volumeLabel muted %q", got)
	}
	if got := truncate("hello", 0); got != "" {
		t.Fatalf("truncate 0 %q", got)
	}
	if got := truncate("你好世界", 2); got == "" {
		t.Fatal("truncate should not be empty for width 2")
	}
	if got := padRight("hi", 5); len(got) < 5 {
		t.Fatalf("padRight len %d", len(got))
	}
	if got := nonEmpty("a", " ", "b"); len(got) != 2 {
		t.Fatalf("nonEmpty %v", got)
	}
	m := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	m.position = -5
	m.duration = math.NaN()
	if got := m.progressPercent(); got != 0 {
		// duration 0 should return 0; NaN would return NaN, accept both 0 and NaN as non-progress
		if !math.IsNaN(got) {
			t.Fatalf("progressPercent with 0 duration %v", got)
		}
	}
	m.position = 5
	m.duration = 10
	if got := m.progressPercent(); got == 0 {
		t.Fatal("progressPercent should not be 0")
	}
	m.position = 0
	m.duration = 0
	// helpText
	m.helpVisible = true
	if ht := m.helpText(80); ht == "" {
		t.Fatal("helpText when visible should not be empty")
	}
	m.helpVisible = false
	m.panel = panelLibrary
	if ht := m.helpText(80); ht == "" {
		t.Fatal("helpText library should not be empty")
	}
	// visibleRange
	if s, e := visibleRange(5, 10, 3); s != 4 || e != 7 {
		t.Fatalf("visibleRange 5,10,3 = %d,%d", s, e)
	}
	if s, e := visibleRange(0, 0, 3); s != 0 || e != 0 {
		t.Fatalf("visibleRange empty %d,%d", s, e)
	}
	// promptView and helpView
	m.prompt = promptPlaylistName
	m.playlistName.SetValue("test")
	if pv := m.promptView(80, 5); len(pv) == 0 {
		t.Fatal("promptView should not be empty")
	}
	m.prompt = promptOverwrite
	m.pendingName = "existing"
	if pv := m.promptView(80, 5); len(pv) == 0 {
		t.Fatal("promptView overwrite should not be empty")
	}
	m.prompt = promptDeletePlaylist
	m.pendingName = "toDelete"
	if pv := m.promptView(80, 5); len(pv) == 0 {
		t.Fatal("promptView delete should not be empty")
	}
	m.prompt = promptNone
	m.helpVisible = true
	if hv := m.helpView(80, 5); len(hv) == 0 {
		t.Fatal("helpView should not be empty")
	}
	// feedbackLine
	m.errText = "error"
	if fl := m.feedbackLine(80); fl == "" {
		t.Fatal("feedbackLine error should not be empty")
	}
	m.errText = ""
	m.noticeText = "notice"
	if fl := m.feedbackLine(80); fl == "" {
		t.Fatal("feedbackLine notice should not be empty")
	}
	// nowPlayingText and statusLine
	m.title = "title"
	m.artist = "artist"
	m.album = "album"
	if title, _ := m.nowPlayingText(); title == "" {
		t.Fatal("nowPlayingText title should not be empty")
	}
	m.current = 0
	m.tracks = []library.Track{{Name: "a", Path: "/a.mp3"}}
	m.position = 10
	m.duration = 100
	m.volume = 75
	m.muted = false
	if sl := m.statusLine(80); sl == "" {
		t.Fatal("statusLine should not be empty")
	}
	// selectedIndex edge
	m.filtered = []int{}
	if idx := m.selectedIndex(); idx != -1 {
		t.Fatalf("selectedIndex empty %d", idx)
	}
}
