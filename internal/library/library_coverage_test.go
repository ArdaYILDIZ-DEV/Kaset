package library

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanWithIssuesAbsoluteAndNotExist(t *testing.T) {
	_, _, err := ScanWithIssues("/nonexistent/path/xyz")
	if err == nil {
		t.Fatal("nonexistent path should return error")
	}
	tmp := t.TempDir()
	file := filepath.Join(tmp, "file.mp3")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err = ScanWithIssues(file)
	if err == nil {
		t.Fatal("file path should return error")
	}
}

func TestScanWithIssuesRelativeAndFolder(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	sub := filepath.Join(tmp, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "a.mp3"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "b.flac"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	tracks, issues, err := ScanWithIssues(".")
	if err != nil {
		t.Fatalf("relative Scan err=%v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("issues %v", issues)
	}
	if len(tracks) != 2 {
		t.Fatalf("tracks len %d want 2", len(tracks))
	}
	foundRoot, foundSub := false, false
	for _, tr := range tracks {
		if tr.Name == "a" && tr.Folder == "" {
			foundRoot = true
		}
		if tr.Name == "b" && tr.Folder == "sub" {
			foundSub = true
		}
	}
	if !foundRoot || !foundSub {
		t.Fatalf("folder field incorrect: %#v", tracks)
	}
}

func TestScanSkipsUnsupportedAndSymlink(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "b.mp3")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "c.mp3")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	tracks, _, err := ScanWithIssues(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 || tracks[0].Path != target {
		t.Fatalf("symlink should be skipped: %#v", tracks)
	}
}
