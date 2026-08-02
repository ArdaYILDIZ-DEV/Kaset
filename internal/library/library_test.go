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
	if tracks[0].Name != "a" || tracks[0].Folder != "Albüm" || tracks[1].Name != "b" || tracks[1].Folder != "" {
		t.Fatalf("track order/metadata = %#v", tracks)
	}
}

func TestScanWithIssuesSkipsUnreadableSubdirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read directories regardless of permission bits")
	}
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(blocked, "hidden.mp3"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "visible.mp3"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) })

	tracks, issues, err := ScanWithIssues(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 || tracks[0].Name != "visible" {
		t.Fatalf("tracks = %#v", tracks)
	}
	if len(issues) != 1 || issues[0].Path != blocked {
		t.Fatalf("issues = %#v", issues)
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
