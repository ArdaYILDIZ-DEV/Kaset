package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanFindsSupportedFilesRecursively(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "Albüm")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"b.OPUS", filepath.Join("Albüm", "a.mp3"), "not-a-track.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tracks, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(tracks) != 2 {
		t.Fatalf("len(tracks) = %d, want 2", len(tracks))
	}
	if tracks[0].Name != "a" || tracks[1].Name != "b" {
		t.Fatalf("track order/names = %#v", tracks)
	}
}

func TestScanSkipsSymlinkedTracks(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "track.mp3")
	if err := os.WriteFile(target, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "linked.opus")); err != nil {
		t.Fatal(err)
	}

	tracks, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(tracks) != 1 || tracks[0].Path != target {
		t.Fatalf("Scan() tracks = %#v, want only %q", tracks, target)
	}
}

func TestScanRejectsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "song.mp3")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Scan(path); err == nil {
		t.Fatal("Scan() error = nil, want an error for a file path")
	}
}
