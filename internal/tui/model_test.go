package tui

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kaset/internal/library"
	"kaset/internal/player"
	"kaset/internal/playlist"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestStartupLogoAnimationSettlesOnAccentColor(t *testing.T) {
	model := New([]library.Track{{Name: "Track", Path: "/track.mp3"}}, nil)
	for letter := range []rune(appName) {
		if got := startupLogoStyleIndex(model.startupFrame, letter); got != 0 {
			t.Fatalf("initial logo style %d = %d, want 0", letter, got)
		}
	}
	for letter, want := range []int{4, 3, 2, 1, 0} {
		if got := startupLogoStyleIndex(2, letter); got != want {
			t.Fatalf("middle logo style %d = %d, want %d", letter, got, want)
		}
	}

	for frame := 1; frame <= startupAnimationLastFrame; frame++ {
		updated, cmd := model.Update(startupTickMsg{})
		model = updated.(Model)
		if model.startupFrame != frame {
			t.Fatalf("startup frame = %d, want %d", model.startupFrame, frame)
		}
		if frame < startupAnimationLastFrame && cmd == nil {
			t.Fatalf("startup frame %d did not schedule the next frame", frame)
		}
		if frame == startupAnimationLastFrame && cmd != nil {
			t.Fatal("final startup frame scheduled another frame")
		}
	}
	for letter := range []rune(appName) {
		if got := startupLogoStyleIndex(model.startupFrame, letter); got != len(startupLogoStyles)-1 {
			t.Fatalf("final logo style %d = %d", letter, got)
		}
	}
}

func TestInitialVolumeOption(t *testing.T) {
	volume := 42.0
	model := NewWithOptions([]library.Track{{Name: "Track", Path: "/track.mp3"}}, nil, Options{
		InitialVolume: &volume,
		ShowFolders:   true,
		HideSidePanel: true,
	})
	if model.Volume() != 42 || !model.ShowFolders() || model.SidePanelEnabled() {
		t.Fatalf("Volume() = %v, ShowFolders() = %v, SidePanelEnabled() = %v", model.Volume(), model.ShowFolders(), model.SidePanelEnabled())
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		seconds float64
		want    string
	}{
		{0, "0:00"},
		{65.9, "1:05"},
		{3600, "60:00"},
		{-1, "0:00"},
		{math.NaN(), "0:00"},
	}
	for _, test := range tests {
		if got := formatDuration(test.seconds); got != test.want {
			t.Errorf("formatDuration(%v) = %q, want %q", test.seconds, got, test.want)
		}
	}
}

func TestVisibleRange(t *testing.T) {
	start, end := visibleRange(8, 10, 4)
	if start != 6 || end != 10 {
		t.Fatalf("visibleRange() = %d, %d; want 6, 10", start, end)
	}

	start, end = visibleRange(1, 3, 5)
	if start != 0 || end != 3 {
		t.Fatalf("visibleRange() = %d, %d; want 0, 3", start, end)
	}
}

func TestSanitizeRemovesTerminalControls(t *testing.T) {
	if got := sanitize("safe\x1b[31m\nname"); got != "safe[31mname" {
		t.Fatalf("sanitize() = %q", got)
	}
}

func TestSearchFirstEscapeClosesSecondEscapeClears(t *testing.T) {
	model := New([]library.Track{
		{Name: "Demo Track", Path: "/music/demo.mp3"},
		{Name: "Başka Şarkı", Path: "/music/baska.opus"},
	}, nil)

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("demo")})
	if !model.searching || model.search.Value() != "demo" || len(model.filtered) != 1 {
		t.Fatalf("search state = focused:%v query:%q filtered:%v", model.searching, model.search.Value(), model.filtered)
	}

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.searching || model.search.Value() != "demo" || len(model.filtered) != 1 {
		t.Fatalf("first Esc cleared search unexpectedly: focused:%v query:%q filtered:%v", model.searching, model.search.Value(), model.filtered)
	}

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	if model.search.Value() != "" || len(model.filtered) != 2 {
		t.Fatalf("second Esc did not clear search: query:%q filtered:%v", model.search.Value(), model.filtered)
	}
}

func TestListAreaToggle(t *testing.T) {
	model := New([]library.Track{{Name: "Track", Path: "/track.mp3"}}, nil)
	model.width = 120
	model.height = 24
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	view := model.View()
	if model.listsVisible || strings.Contains(view, "KÜTÜPHANE") || strings.Contains(view, "ÇALMA SIRASI") {
		t.Fatalf("list area remained visible after t: %q", view)
	}

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if !model.listsVisible || !model.sidePanelEnabled || model.panel != panelQueue {
		t.Fatalf("y did not open the queue from the hidden list view: lists=%v side=%v panel=%v", model.listsVisible, model.sidePanelEnabled, model.panel)
	}

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if !model.listsVisible || !model.sidePanelEnabled || model.panel != panelQueue {
		t.Fatal("t did not restore the previous list state")
	}
}

func TestQueueSidePanelToggleDoesNotFollowTrackSelection(t *testing.T) {
	controller := &fakePlayer{events: make(chan player.Event)}
	model := New([]library.Track{{Name: "Track", Path: "/track.mp3"}}, controller)
	model.width = 120
	model.height = 24

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	view := model.View()
	if model.sidePanelEnabled || model.panel != panelLibrary || strings.Contains(view, "ÇALMA SIRASI") {
		t.Fatalf("queue panel did not close: enabled=%v panel=%v view=%q", model.sidePanelEnabled, model.panel, view)
	}

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.sidePanelEnabled || len(controller.loads) != 1 {
		t.Fatalf("track selection reopened queue: enabled=%v loads=%v", model.sidePanelEnabled, controller.loads)
	}

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyTab})
	if model.sidePanelEnabled || model.panel != panelLibrary {
		t.Fatalf("Tab opened a hidden side panel: enabled=%v panel=%v", model.sidePanelEnabled, model.panel)
	}

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	view = model.View()
	if !model.sidePanelEnabled || model.sidePanel != panelQueue || model.panel != panelQueue || !strings.Contains(view, "ÇALMA SIRASI") {
		t.Fatalf("queue panel did not reopen: enabled=%v side=%v panel=%v view=%q", model.sidePanelEnabled, model.sidePanel, model.panel, view)
	}
}

func TestSearchResultsBecomePlaybackQueue(t *testing.T) {
	controller := &fakePlayer{events: make(chan player.Event)}
	model := New([]library.Track{
		{Name: "Other", Path: "/music/other.mp3"},
		{Name: "Demo One", Path: "/music/demo-one.mp3"},
		{Name: "Demo Two", Path: "/music/demo-two.mp3"},
	}, controller)
	model.search.SetValue("demo")
	model.refreshFilter()
	model.playLibraryContext()

	if got := model.queue.Items(); !equalInts(got, []int{1, 2}) {
		t.Fatalf("queue = %v, want [1 2]", got)
	}
	if model.queue.Position() != 0 || len(controller.loads) != 1 || controller.loads[0] != "/music/demo-one.mp3" {
		t.Fatalf("initial playback position=%d loads=%v", model.queue.Position(), controller.loads)
	}

	model.handlePlayerEvent(player.Event{Type: player.EventEndFile, Reason: "eof"})
	if model.queue.Position() != 1 || len(controller.loads) != 2 || controller.loads[1] != "/music/demo-two.mp3" {
		t.Fatalf("next playback position=%d loads=%v", model.queue.Position(), controller.loads)
	}
	model.handlePlayerEvent(player.Event{Type: player.EventEndFile, Reason: "eof"})
	if model.current != -1 {
		t.Fatalf("current after queue end = %d, want -1", model.current)
	}
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	if len(controller.loads) != 3 || controller.loads[2] != "/music/demo-two.mp3" {
		t.Fatalf("Space after queue end loads=%v", controller.loads)
	}
}

func TestLoopRepeatsCurrentQueueTrackAndTogglesOff(t *testing.T) {
	controller := &fakePlayer{events: make(chan player.Event)}
	model := New([]library.Track{
		{Name: "One", Path: "/music/one.mp3"},
		{Name: "Two", Path: "/music/two.mp3"},
	}, controller)
	model.queue.Replace([]int{0, 1}, -1)
	if !model.playQueueAt(0) {
		t.Fatal("initial play failed")
	}

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if !model.loopCurrent || !strings.Contains(model.View(), "PARÇA DÖNGÜSÜ") {
		t.Fatal("loop did not turn on")
	}
	model.handlePlayerEvent(player.Event{Type: player.EventEndFile, Reason: "eof"})
	if model.queue.Position() != 0 || len(controller.loads) != 2 || controller.loads[1] != "/music/one.mp3" {
		t.Fatalf("loop playback position=%d loads=%v", model.queue.Position(), controller.loads)
	}

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if model.loopCurrent {
		t.Fatal("loop did not turn off")
	}
	model.handlePlayerEvent(player.Event{Type: player.EventEndFile, Reason: "eof"})
	if model.queue.Position() != 1 || len(controller.loads) != 3 || controller.loads[2] != "/music/two.mp3" {
		t.Fatalf("playback after loop position=%d loads=%v", model.queue.Position(), controller.loads)
	}
}

func TestLoopRequiresCurrentTrack(t *testing.T) {
	model := New([]library.Track{{Name: "One", Path: "/music/one.mp3"}}, nil)
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if model.loopCurrent || !strings.Contains(model.errText, "çalan bir parça yok") {
		t.Fatalf("loop=%v error=%q", model.loopCurrent, model.errText)
	}
}

func TestPlaylistRoundTripLoadsDirectPathsIntoQueue(t *testing.T) {
	store := playlist.NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	tracks := []library.Track{
		{Name: "One", Path: "/music/one.mp3"},
		{Name: "Two", Path: "/music/two.opus"},
	}
	model := New(tracks, nil, store)
	model.queue.Replace([]int{1, 0}, -1)
	model.playlistName.SetValue("Karışık")
	model.submitPlaylistName()
	if model.errText != "" || model.loadedPlaylist != "Karışık" {
		t.Fatalf("save result: error=%q loaded=%q", model.errText, model.loadedPlaylist)
	}

	loaded := New(tracks, nil, store)
	loaded.openPlaylists()
	if len(loaded.playlists) != 1 {
		t.Fatalf("playlists = %#v", loaded.playlists)
	}
	loaded.loadSelectedPlaylist()
	if got := loaded.queue.Items(); !equalInts(got, []int{1, 0}) {
		t.Fatalf("loaded queue = %v, want [1 0]", got)
	}
	if loaded.panel != panelQueue || loaded.queue.Position() != -1 {
		t.Fatalf("loaded panel=%v position=%d", loaded.panel, loaded.queue.Position())
	}
}

func TestLoopWorksForTrackLoadedFromPlaylist(t *testing.T) {
	store := playlist.NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	tracks := []library.Track{
		{Name: "One", Path: "/music/one.mp3"},
		{Name: "Two", Path: "/music/two.mp3"},
	}
	if err := store.Save("Mix", []string{tracks[1].Path, tracks[0].Path}, false); err != nil {
		t.Fatal(err)
	}
	controller := &fakePlayer{events: make(chan player.Event)}
	model := New(tracks, controller, store)
	model.openPlaylists()
	model.loadSelectedPlaylist()
	if !model.playQueueAt(0) {
		t.Fatal("playlist track did not start")
	}
	model.toggleCurrentLoop()
	model.handlePlayerEvent(player.Event{Type: player.EventEndFile, Reason: "eof"})
	if !model.loopCurrent || model.queue.Position() != 0 || len(controller.loads) != 2 {
		t.Fatalf("loop=%v position=%d loads=%v", model.loopCurrent, model.queue.Position(), controller.loads)
	}
	if controller.loads[0] != tracks[1].Path || controller.loads[1] != tracks[1].Path {
		t.Fatalf("playlist loop loads=%v", controller.loads)
	}
}

func TestOpeningPlaylistsRecoversInvalidStore(t *testing.T) {
	store := playlist.NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	if err := os.WriteFile(store.Path(), []byte(`{"version":99,"playlists":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	model := New([]library.Track{{Name: "One", Path: "/music/one.mp3"}}, nil, store)
	model.openPlaylists()
	if model.panel != panelPlaylists || len(model.playlists) != 0 {
		t.Fatalf("panel=%v playlists=%#v", model.panel, model.playlists)
	}
	if !strings.Contains(model.noticeText, "yedeklendi") {
		t.Fatalf("notice = %q", model.noticeText)
	}
}

func TestSavingPlaylistRetriesAfterStoreRecovery(t *testing.T) {
	store := playlist.NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	if err := os.WriteFile(store.Path(), []byte(`{"version":99,"playlists":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	track := library.Track{Name: "One", Path: "/music/one.mp3"}
	model := New([]library.Track{track}, nil, store)
	model.queue.Replace([]int{0}, -1)
	model.playlistName.SetValue("Recovered")
	model.submitPlaylistName()

	if model.errText != "" || model.loadedPlaylist != "Recovered" {
		t.Fatalf("save result: error=%q loaded=%q", model.errText, model.loadedPlaylist)
	}
	if _, err := store.Load("Recovered"); err != nil {
		t.Fatalf("recovered playlist was not saved: %v", err)
	}
	if !strings.Contains(model.noticeText, "yedeklendi") || !strings.Contains(model.noticeText, "kaydedildi") {
		t.Fatalf("notice = %q", model.noticeText)
	}
}

func TestExistingPlaylistRequiresOverwriteConfirmation(t *testing.T) {
	store := playlist.NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	tracks := []library.Track{{Name: "One", Path: "/music/one.mp3"}}
	if err := store.Save("Mix", []string{tracks[0].Path}, false); err != nil {
		t.Fatal(err)
	}
	model := New(tracks, nil, store)
	model.queue.Replace([]int{0}, -1)
	model.playlistName.SetValue("Mix")
	model.submitPlaylistName()
	if model.prompt != promptOverwrite || model.pendingName != "Mix" {
		t.Fatalf("prompt=%v pending=%q", model.prompt, model.pendingName)
	}
}

func TestPlaylistDeleteRequiresConfirmationAndKeepsQueue(t *testing.T) {
	store := playlist.NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	track := "/music/one.mp3"
	if err := store.Save("Delete", []string{track}, false); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("Keep", []string{track}, false); err != nil {
		t.Fatal(err)
	}
	model := New([]library.Track{{Name: "One", Path: track}}, nil, store)
	model.queue.Replace([]int{0}, -1)
	model.loadedPlaylist = "Delete"
	model.openPlaylists()

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if model.prompt != promptDeletePlaylist || model.pendingName != "Delete" {
		t.Fatalf("delete prompt=%v pending=%q", model.prompt, model.pendingName)
	}
	if !strings.Contains(model.View(), "ÇALMA LİSTESİ SİL") {
		t.Fatal("delete confirmation is not visible")
	}
	if _, err := store.Load("Delete"); err != nil {
		t.Fatalf("playlist was deleted before confirmation: %v", err)
	}

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	if _, err := store.Load("Delete"); err != nil {
		t.Fatalf("Esc deleted playlist: %v", err)
	}
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	if _, err := store.Load("Delete"); !errors.Is(err, playlist.ErrNotFound) {
		t.Fatalf("confirmed delete error = %v", err)
	}
	if model.queue.Len() != 1 || model.loadedPlaylist != "" {
		t.Fatalf("queue=%v loadedPlaylist=%q", model.queue.Items(), model.loadedPlaylist)
	}
}

func TestPlaybackErrorSkipsToNextTrack(t *testing.T) {
	controller := &fakePlayer{events: make(chan player.Event)}
	model := New([]library.Track{
		{Name: "Broken", Path: "/music/broken.mp3"},
		{Name: "Working", Path: "/music/working.mp3"},
	}, controller)
	model.queue.Replace([]int{0, 1}, -1)
	if !model.playQueueAt(0) {
		t.Fatal("initial play failed")
	}

	model.handlePlayerEvent(player.Event{Type: player.EventEndFile, Reason: "error", FileError: "unrecognized file format"})
	if model.queue.Position() != 1 || model.current != 1 || len(controller.loads) != 2 {
		t.Fatalf("position=%d current=%d loads=%v", model.queue.Position(), model.current, controller.loads)
	}
	if !strings.Contains(model.noticeText, "unrecognized file format") || !strings.Contains(model.noticeText, "sıradaki parçaya geçildi") {
		t.Fatalf("notice = %q", model.noticeText)
	}
}

func TestRemovingActiveQueueItemKeepsPlaybackAndNavigation(t *testing.T) {
	controller := &fakePlayer{events: make(chan player.Event)}
	model := New([]library.Track{
		{Name: "Previous", Path: "/music/previous.mp3"},
		{Name: "Current", Path: "/music/current.mp3"},
		{Name: "Next", Path: "/music/next.mp3"},
	}, controller)
	model.queue.Replace([]int{0, 1, 2}, -1)
	if !model.playQueueAt(1) {
		t.Fatal("initial play failed")
	}
	model.queueCursor = 1
	model.removeQueueItem()
	if model.current != 1 || model.queue.Position() != -1 {
		t.Fatalf("current=%d position=%d", model.current, model.queue.Position())
	}

	model.handlePlayerEvent(player.Event{Type: player.EventEndFile, Reason: "eof"})
	if model.current != 2 || len(controller.loads) != 2 || controller.loads[1] != "/music/next.mp3" {
		t.Fatalf("current=%d loads=%v", model.current, controller.loads)
	}
}

func TestQueueRepeatWrapsAtEnd(t *testing.T) {
	controller := &fakePlayer{events: make(chan player.Event)}
	model := New([]library.Track{
		{Name: "One", Path: "/music/one.mp3"},
		{Name: "Two", Path: "/music/two.mp3"},
	}, controller)
	model.queue.Replace([]int{0, 1}, -1)
	if !model.playQueueAt(1) {
		t.Fatal("initial play failed")
	}
	model.toggleQueueRepeat()
	model.handlePlayerEvent(player.Event{Type: player.EventEndFile, Reason: "eof"})
	if !model.repeatQueue || model.queue.Position() != 0 || controller.loads[1] != "/music/one.mp3" {
		t.Fatalf("repeat=%v position=%d loads=%v", model.repeatQueue, model.queue.Position(), controller.loads)
	}
}

func TestLibraryRefreshRemapsQueueAndCurrentTrackByPath(t *testing.T) {
	controller := &fakePlayer{events: make(chan player.Event)}
	model := NewWithOptions([]library.Track{
		{Name: "One", Path: "/music/one.mp3"},
		{Name: "Two", Path: "/music/two.mp3"},
	}, controller, Options{LibraryRoot: "/music"})
	model.queue.Replace([]int{0, 1}, -1)
	if !model.playQueueAt(1) {
		t.Fatal("initial play failed")
	}

	model.applyLibraryScan(libraryScanMsg{tracks: []library.Track{
		{Name: "Two", Path: "/music/two.mp3"},
		{Name: "Three", Path: "/music/three.mp3"},
	}})
	if model.current != 0 || model.queue.Position() != 0 || !equalInts(model.queue.Items(), []int{0}) {
		t.Fatalf("current=%d position=%d queue=%v", model.current, model.queue.Position(), model.queue.Items())
	}
	if !strings.Contains(model.noticeText, "1 eksik parça") {
		t.Fatalf("notice = %q", model.noticeText)
	}
}

func TestLibraryRefreshPreservesDetachedNextTrack(t *testing.T) {
	controller := &fakePlayer{events: make(chan player.Event)}
	model := NewWithOptions([]library.Track{
		{Name: "Previous", Path: "/music/previous.mp3"},
		{Name: "Current", Path: "/music/current.mp3"},
		{Name: "Next", Path: "/music/next.mp3"},
		{Name: "Later", Path: "/music/later.mp3"},
	}, controller, Options{LibraryRoot: "/music"})
	model.queue.Replace([]int{0, 1, 2, 3}, -1)
	if !model.playQueueAt(1) {
		t.Fatal("initial play failed")
	}
	model.queueCursor = 1
	model.removeQueueItem()

	model.applyLibraryScan(libraryScanMsg{tracks: []library.Track{
		{Name: "Later", Path: "/music/later.mp3"},
		{Name: "Next", Path: "/music/next.mp3"},
		{Name: "Current", Path: "/music/current.mp3"},
		{Name: "Previous", Path: "/music/previous.mp3"},
	}})
	if got := model.queue.Items(); !equalInts(got, []int{3, 1, 0}) {
		t.Fatalf("queue = %v, want [3 1 0]", got)
	}
	if next, ok := model.queue.NextPosition(); !ok || next != 1 {
		t.Fatalf("NextPosition() = %d, %v; want 1, true", next, ok)
	}

	model.handlePlayerEvent(player.Event{Type: player.EventEndFile, Reason: "eof"})
	if model.current != 1 || model.queue.Position() != 1 || len(controller.loads) != 2 || controller.loads[1] != "/music/next.mp3" {
		t.Fatalf("current=%d position=%d loads=%v", model.current, model.queue.Position(), controller.loads)
	}
}

func TestLibraryRefreshSkipsMissingDetachedNextTrack(t *testing.T) {
	controller := &fakePlayer{events: make(chan player.Event)}
	model := NewWithOptions([]library.Track{
		{Name: "Previous", Path: "/music/previous.mp3"},
		{Name: "Current", Path: "/music/current.mp3"},
		{Name: "Missing Next", Path: "/music/missing-next.mp3"},
		{Name: "Later", Path: "/music/later.mp3"},
	}, controller, Options{LibraryRoot: "/music"})
	model.queue.Replace([]int{0, 1, 2, 3}, -1)
	if !model.playQueueAt(1) {
		t.Fatal("initial play failed")
	}
	model.queueCursor = 1
	model.removeQueueItem()

	model.applyLibraryScan(libraryScanMsg{tracks: []library.Track{
		{Name: "Later", Path: "/music/later.mp3"},
		{Name: "Current", Path: "/music/current.mp3"},
		{Name: "Previous", Path: "/music/previous.mp3"},
	}})
	if next, ok := model.queue.NextPosition(); !ok || next != 1 {
		t.Fatalf("NextPosition() = %d, %v; want 1, true", next, ok)
	}

	model.handlePlayerEvent(player.Event{Type: player.EventEndFile, Reason: "eof"})
	if model.current != 0 || len(controller.loads) != 2 || controller.loads[1] != "/music/later.mp3" {
		t.Fatalf("current=%d loads=%v", model.current, controller.loads)
	}
}

func TestSearchHandlesTurkishCase(t *testing.T) {
	model := New([]library.Track{
		{Name: "Işık", Path: "/music/isik.mp3"},
		{Name: "Irmak", Path: "/music/irmak.mp3"},
	}, nil)
	model.search.SetValue("ışık")
	model.refreshFilter()
	if !equalInts(model.filtered, []int{0}) {
		t.Fatalf("filtered = %v", model.filtered)
	}
}

func TestPlaylistLoadSkipsPathsOutsideCurrentLibrary(t *testing.T) {
	store := playlist.NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	if err := store.Save("Partial", []string{"/music/one.mp3", "/other/missing.flac"}, false); err != nil {
		t.Fatal(err)
	}
	model := New([]library.Track{{Name: "One", Path: "/music/one.mp3"}}, nil, store)
	model.openPlaylists()
	model.loadSelectedPlaylist()
	if got := model.queue.Items(); !equalInts(got, []int{0}) {
		t.Fatalf("loaded queue = %v", got)
	}
	if !strings.Contains(model.noticeText, "1 eksik dosya") {
		t.Fatalf("notice = %q", model.noticeText)
	}
}

func TestHelpAndWideLayout(t *testing.T) {
	model := New([]library.Track{{Name: "Track", Path: "/music/track.mp3", Folder: "Album"}}, nil)
	model.width = 120
	model.height = 24
	view := model.View()
	if !strings.Contains(view, "KÜTÜPHANE") || !strings.Contains(view, "ÇALMA SIRASI") {
		t.Fatalf("wide view does not contain both panels: %q", view)
	}
	if strings.Contains(view, "Album") {
		t.Fatalf("folder details are visible by default: %q", view)
	}
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !model.ShowFolders() || !strings.Contains(model.View(), "Track :: Album") || !strings.Contains(model.noticeText, "gösteriliyor") {
		t.Fatalf("folder details did not become visible: show=%v notice=%q view=%q", model.ShowFolders(), model.noticeText, model.View())
	}
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	help := model.View()
	if !model.helpVisible || !strings.Contains(help, "YARDIM") || !strings.Contains(help, "Tab odağı") || !strings.Contains(help, "d klasör ayrıntısı") || !strings.Contains(help, "y çalma sırası") || !strings.Contains(help, "t tüm listeleri gizle") {
		t.Fatal("help view did not open with panel and detail instructions")
	}
}

func TestWideLayoutFillsAvailableRowsWithDivider(t *testing.T) {
	model := New([]library.Track{{Name: "Track", Path: "/music/track.mp3"}}, nil)
	model.width = 120

	const available = 8
	lines := model.wideLibraryView(model.width, available, panelQueue)
	if len(lines) != available {
		t.Fatalf("wide layout rows = %d, want %d", len(lines), available)
	}
	for row, line := range lines {
		if !strings.Contains(line, "│") {
			t.Fatalf("wide layout row %d has no divider: %q", row, line)
		}
	}
}

func TestWideLayoutTabChangesVisiblePanelFocus(t *testing.T) {
	model := New([]library.Track{
		{Name: "Library Track", Path: "/music/library.mp3"},
		{Name: "Queued Track", Path: "/music/queued.mp3"},
	}, nil)
	model.queue.Append(1)
	model.width = 120
	model.height = 24

	view := model.View()
	if !strings.Contains(view, "● KÜTÜPHANE") || !strings.Contains(view, "○ ÇALMA SIRASI") {
		t.Fatalf("library focus is not visible: %q", view)
	}
	if got := strings.Count(view, "> "); got != 1 {
		t.Fatalf("active cursor count = %d, want 1", got)
	}

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyTab})
	view = model.View()
	if model.panel != panelQueue || !strings.Contains(view, "○ KÜTÜPHANE") || !strings.Contains(view, "● ÇALMA SIRASI") {
		t.Fatalf("queue focus is not visible: panel=%v view=%q", model.panel, view)
	}
	if got := strings.Count(view, "> "); got != 1 {
		t.Fatalf("active cursor count after Tab = %d, want 1", got)
	}
	if !strings.Contains(model.noticeText, "Odak: Çalma sırası") || !strings.Contains(view, "Tab → kütüphane") {
		t.Fatalf("focus guidance missing: notice=%q view=%q", model.noticeText, view)
	}
}

func TestWidePlaylistsStayInRightPanelAndReturnToQueueAfterLoad(t *testing.T) {
	store := playlist.NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	track := library.Track{Name: "Library Track", Path: "/music/track.mp3"}
	if err := store.Save("Mix", []string{track.Path}, false); err != nil {
		t.Fatal(err)
	}
	model := New([]library.Track{track}, nil, store)
	model.width = 120
	model.height = 24

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	view := model.View()
	if model.panel != panelPlaylists || !strings.Contains(view, "○ KÜTÜPHANE") || !strings.Contains(view, "● ÇALMA LİSTELERİ") {
		t.Fatalf("wide playlist panel is not visible beside library: panel=%v view=%q", model.panel, view)
	}
	if !strings.Contains(view, "Library Track") || strings.Contains(view, "ÇALMA SIRASI") {
		t.Fatalf("unexpected wide playlist content: %q", view)
	}

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyTab})
	view = model.View()
	if model.panel != panelLibrary || model.sidePanel != panelPlaylists || !strings.Contains(view, "● KÜTÜPHANE") || !strings.Contains(view, "○ ÇALMA LİSTELERİ") {
		t.Fatalf("Tab did not preserve the visible playlist panel: panel=%v side=%v view=%q", model.panel, model.sidePanel, view)
	}
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if model.sidePanelEnabled || model.panel != panelLibrary {
		t.Fatalf("P did not close playlists: enabled=%v panel=%v", model.sidePanelEnabled, model.panel)
	}
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if !model.listsVisible || !model.sidePanelEnabled || model.sidePanel != panelPlaylists || model.panel != panelPlaylists {
		t.Fatalf("P did not open playlists from the hidden list view: lists=%v enabled=%v side=%v panel=%v", model.listsVisible, model.sidePanelEnabled, model.sidePanel, model.panel)
	}

	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	view = model.View()
	if model.panel != panelQueue || !strings.Contains(view, "○ KÜTÜPHANE") || !strings.Contains(view, "● ÇALMA SIRASI") {
		t.Fatalf("queue did not replace playlists after load: panel=%v view=%q", model.panel, view)
	}
	if model.loadedPlaylist != "Mix" || !equalInts(model.queue.Items(), []int{0}) {
		t.Fatalf("loaded=%q queue=%v", model.loadedPlaylist, model.queue.Items())
	}
}

func TestNarrowPlaylistsKeepSinglePanelLayout(t *testing.T) {
	store := playlist.NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	track := library.Track{Name: "Library Track", Path: "/music/track.mp3"}
	if err := store.Save("Mix", []string{track.Path}, false); err != nil {
		t.Fatal(err)
	}
	model := New([]library.Track{track}, nil, store)
	model.width = 80
	model.height = 24
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	view := model.View()
	if !strings.Contains(view, "ÇALMA LİSTELERİ") || strings.Contains(view, "KÜTÜPHANE") {
		t.Fatalf("narrow playlist view is not single-panel: %q", view)
	}
}

func TestTruncateUsesTerminalCellWidth(t *testing.T) {
	for width := 1; width <= 8; width++ {
		if got := lipgloss.Width(truncate("你好🙂e\u0301", width)); got > width {
			t.Fatalf("truncate width = %d, want <= %d", got, width)
		}
	}
}

func TestViewUsesKasetBrand(t *testing.T) {
	model := New([]library.Track{{Name: "Track", Path: "/track.mp3"}}, nil)
	view := model.View()
	if !strings.Contains(view, "KASET") || strings.Contains(view, "TERM"+"USIC") {
		t.Fatalf("unexpected application branding in View(): %q", view)
	}
}

func TestViewFitsNarrowWindows(t *testing.T) {
	model := New([]library.Track{{Name: "A very long track name that must shrink", Path: "/track.mp3"}}, nil)
	model.queue.Append(0)
	for _, panel := range []panelKind{panelLibrary, panelQueue, panelPlaylists} {
		model.panel = panel
		for _, size := range []struct{ width, height int }{{1, 8}, {2, 8}, {8, 1}, {12, 2}, {20, 5}, {30, 8}, {80, 24}} {
			model.width = size.width
			model.height = size.height
			lines := strings.Split(model.View(), "\n")
			if len(lines) != size.height {
				t.Errorf("View() panel %d at %dx%d has %d lines", panel, size.width, size.height, len(lines))
			}
			for index, line := range lines {
				if got := lipgloss.Width(line); got > size.width {
					t.Errorf("View() panel %d at %dx%d line %d width = %d", panel, size.width, size.height, index, got)
				}
			}
		}
	}
}

type fakePlayer struct {
	events chan player.Event
	loads  []string
}

func (f *fakePlayer) Events() <-chan player.Event { return f.events }
func (f *fakePlayer) Load(path string) error {
	f.loads = append(f.loads, path)
	return nil
}
func (f *fakePlayer) TogglePause() error         { return nil }
func (f *fakePlayer) Stop() error                { return nil }
func (f *fakePlayer) Seek(float64) error         { return nil }
func (f *fakePlayer) ChangeVolume(float64) error { return nil }
func (f *fakePlayer) ToggleMute() error          { return nil }

func equalInts(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func updateWithKey(t *testing.T, model Model, key tea.KeyMsg) Model {
	t.Helper()
	updated, _ := model.Update(key)
	result, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update() returned %T", updated)
	}
	return result
}
