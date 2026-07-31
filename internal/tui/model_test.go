package tui

import (
	"errors"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"kaset/internal/library"
	"kaset/internal/player"
	"kaset/internal/playlist"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

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

func TestLibraryToggle(t *testing.T) {
	model := New([]library.Track{{Name: "Track", Path: "/track.mp3"}}, nil)
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if model.libraryVisible {
		t.Fatal("library remained visible after t")
	}
	model = updateWithKey(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if !model.libraryVisible {
		t.Fatal("library remained hidden after second t")
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
	if !model.loopCurrent || !strings.Contains(model.View(), "LOOP") {
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
	if model.loopCurrent || !strings.Contains(model.errText, "çalan bir şarkı yok") {
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
	model.toggleLoop()
	model.handlePlayerEvent(player.Event{Type: player.EventEndFile, Reason: "eof"})
	if !model.loopCurrent || model.queue.Position() != 0 || len(controller.loads) != 2 {
		t.Fatalf("loop=%v position=%d loads=%v", model.loopCurrent, model.queue.Position(), controller.loads)
	}
	if controller.loads[0] != tracks[1].Path || controller.loads[1] != tracks[1].Path {
		t.Fatalf("playlist loop loads=%v", controller.loads)
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
	if !strings.Contains(model.View(), "PLAYLIST SİL") {
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
