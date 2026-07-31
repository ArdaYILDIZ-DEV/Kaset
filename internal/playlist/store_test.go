package playlist

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveLoadAndOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "playlists.json")
	store := NewStore(path)
	first := []string{filepath.Join(t.TempDir(), "one.mp3"), filepath.Join(t.TempDir(), "two.opus")}
	if err := store.Save("Gece", first, false); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("Gece", []string{first[1]}, false); !errors.Is(err, ErrExists) {
		t.Fatalf("Save(existing) error = %v", err)
	}
	if err := store.Save("Gece", []string{first[1]}, true); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load("Gece")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(loaded.Tracks, []string{first[1]}) {
		t.Fatalf("Load().Tracks = %v", loaded.Tracks)
	}
}

func TestListSortsAndReturnsCopies(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	track := filepath.Join(t.TempDir(), "track.mp3")
	if err := store.Save("Zaman", []string{track}, false); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("Akşam", []string{track}, false); err != nil {
		t.Fatal(err)
	}

	items, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Name != "Akşam" || items[1].Name != "Zaman" {
		t.Fatalf("List() = %#v", items)
	}
	items[0].Tracks[0] = "changed"
	reloaded, err := store.Load("Akşam")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Tracks[0] == "changed" {
		t.Fatal("List() exposed store data")
	}
}

func TestDeletePlaylist(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	track := filepath.Join(t.TempDir(), "track.mp3")
	if err := store.Save("Keep", []string{track}, false); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("Delete", []string{track}, false); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("Delete"); err != nil {
		t.Fatal(err)
	}
	items, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "Keep" {
		t.Fatalf("List() after delete = %#v", items)
	}
	if err := store.Delete("Delete"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete(missing) error = %v", err)
	}
}

func TestFileContainsDirectAbsolutePaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "playlists.json")
	store := NewStore(path)
	if err := store.Save("Test", []string{"relative/song.mp3"}, false); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var data fileData
	if err := json.Unmarshal(content, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Playlists) != 1 || !filepath.IsAbs(data.Playlists[0].Tracks[0]) {
		t.Fatalf("stored paths = %#v", data.Playlists)
	}
}

func TestDefaultStoreUsesKasetConfigPath(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	store, err := DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(configDir, "kaset", "playlists.json")
	if store.Path() != expected {
		t.Fatalf("Path() = %q, want %q", store.Path(), expected)
	}

	track := filepath.Join(t.TempDir(), "track.mp3")
	if err := store.Save("Test", []string{track}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("kaset store was not created: %v", err)
	}
}

func TestRejectsInvalidData(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "playlists.json"))
	if err := store.Save("  ", []string{"track.mp3"}, false); err == nil {
		t.Fatal("Save() accepted an empty name")
	}
	if err := store.Save("Valid", nil, false); err == nil {
		t.Fatal("Save() accepted an empty playlist")
	}

	if err := os.WriteFile(store.Path(), []byte(`{"version":99,"playlists":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(); err == nil {
		t.Fatal("List() accepted an unsupported version")
	}
}
