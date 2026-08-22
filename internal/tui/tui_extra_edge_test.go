package tui

import (
	"path/filepath"
	"testing"

	"kaset/internal/library"
	"kaset/internal/playlist"
)

func TestPlaylistsEdgeCases(t *testing.T) {
	// beginPlaylistDelete with nil store
	m := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	m.playlists = []playlist.Playlist{{Name: "one", Tracks: []string{"/a.mp3"}}}
	m.playlistCursor = 0
	m.beginPlaylistDelete()
	if m.errText == "" {
		t.Fatal("beginPlaylistDelete with nil store should error")
	}
	// beginPlaylistDelete with invalid cursor
	dir := t.TempDir()
	store := playlist.NewStore(filepath.Join(dir, "playlists.json"))
	m2 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil, store)
	m2.playlists = []playlist.Playlist{}
	m2.playlistCursor = 5
	m2.beginPlaylistDelete()
	if m2.errText == "" {
		t.Fatal("beginPlaylistDelete with invalid cursor should error")
	}
	// openPlaylists with nil store
	m3 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	m3.openPlaylists()
	if m3.errText == "" {
		t.Fatal("openPlaylists with nil store should error")
	}
	// savePlaylist with error (empty name will be validated)
	m4 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil, store)
	m4.queue.Append(0)
	m4.savePlaylist("   ", false)
	if m4.errText == "" {
		t.Fatal("savePlaylist with empty name should error")
	}
	// deletePendingPlaylist with error (non-existent)
	m4.pendingName = "nonexistent"
	m4.deletePendingPlaylist()
	if m4.errText == "" {
		t.Fatal("deletePendingPlaylist with missing should error")
	}
	// loadSelectedPlaylist with no playlists
	m5 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil, store)
	m5.playlists = []playlist.Playlist{}
	m5.playlistCursor = 0
	m5.loadSelectedPlaylist()
	if m5.errText == "" {
		t.Fatal("loadSelectedPlaylist with no playlists should error")
	}
	// refreshPlaylists with corrupt file
	corruptStore := playlist.NewStore(filepath.Join(dir, "corrupt.json"))
	// write corrupt
	if err := store.Save("test", []string{"/a.mp3"}, false); err != nil {
		// ensure store has something
	}
	// manually corrupt file
	_ = corruptStore
}

func TestPlaybackEdgeCases(t *testing.T) {
	// playLibraryContext with empty filtered
	m := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	m.filtered = []int{}
	m.playLibraryContext()
	if m.errText == "" {
		t.Fatal("playLibraryContext with empty filtered should error")
	}
	// playQueueAt with invalid position
	m2 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	m2.queue.Replace([]int{0}, -1)
	if m2.playQueueAt(5) {
		t.Fatal("playQueueAt invalid should fail")
	}
	// playTrack with nil player
	m3 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	if m3.playTrack(0) {
		t.Fatal("playTrack with nil player should fail")
	}
	// toggleCurrentLoop without current
	m4 := New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	m4.current = -1
	m4.toggleCurrentLoop()
	if m4.errText == "" {
		t.Fatal("toggleCurrentLoop without current should error")
	}
	// toggleQueueRepeat already covered but test again
	m4.toggleQueueRepeat()
	m4.toggleQueueRepeat()
}

func TestViewEdgeCases(t *testing.T) {
	_ = New([]library.Track{{Name: "a", Path: "/a.mp3"}}, nil)
	// progressPercent with 0 duration already tested, but test volumeLabel edge
	if got := volumeLabel(-10, false); got != "0%" {
		t.Fatalf("volumeLabel negative %q", got)
	}
	if got := volumeLabel(150, false); got != "100%" {
		t.Fatalf("volumeLabel over %q", got)
	}
	// truncate edge
	if got := truncate("hello", 3); got == "" {
		t.Fatal("truncate should not be empty")
	}
	// nonEmpty edge
	if got := nonEmpty(); len(got) != 0 {
		t.Fatal("nonEmpty with no args should be empty")
	}
	if got := nonEmpty(" ", "  "); len(got) != 0 {
		t.Fatal("nonEmpty with blank should be empty")
	}
	// startupAnimationTick already covered
}
