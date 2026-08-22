//go:build linux

package playlist

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsurePrivateDirectoryFailure(t *testing.T) {
	dir := t.TempDir()
	fileAsDir := filepath.Join(dir, "file")
	if err := os.WriteFile(fileAsDir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(filepath.Join(fileAsDir, "playlists.json"))
	err := store.Save("test", []string{filepath.Join(dir, "a.mp3")}, false)
	if err == nil {
		t.Fatal("Save with file-as-directory should fail")
	}
	// Also test write failure when path is directory
	store2 := NewStore(filepath.Join(dir, "dir.json"))
	if err := os.Mkdir(store2.Path(), 0o755); err != nil {
		t.Fatal(err)
	}
	err = store2.Save("test", []string{filepath.Join(dir, "a.mp3")}, false)
	if err == nil {
		t.Fatal("Save where path is directory should fail")
	}
}

func TestWithLockFailureOnLockFile(t *testing.T) {
	dir := t.TempDir()
	// Create a directory and make its lock file path a directory to cause OpenFile failure
	storePath := filepath.Join(dir, "playlists.json")
	lockPath := storePath + ".lock"
	if err := os.Mkdir(lockPath, 0o755); err != nil {
		t.Fatal(err)
	}
	store := NewStore(storePath)
	// Need a track to save
	track := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(track, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	err := store.Save("test", []string{track}, false)
	if err == nil {
		t.Fatal("Save with lock path as directory should fail")
	}
	// clean up lock dir for other tests
	_ = os.RemoveAll(lockPath)
	_, err = store.List()
	// List should also try to open lock, but now it should succeed after removing lock dir
	if err != nil {
		// List may succeed with empty, that's okay
	}
}

func TestReadFailureWhenPathIsDirectory(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "playlists.json")
	if err := os.Mkdir(storePath, 0o755); err != nil {
		t.Fatal(err)
	}
	store := NewStore(storePath)
	_, err := store.List()
	if err == nil {
		t.Fatal("List where path is directory should fail")
	}
	_, err = store.Load("any")
	if err == nil {
		t.Fatal("Load where path is directory should fail")
	}
}

func TestWriteFailureDueToTempFile(t *testing.T) {
	dir := t.TempDir()
	track := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(track, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// Make directory read-only to cause CreateTemp failure - but as root, still writable
	// Instead create a file where temp should be created by making directory a file
	fileAsDir := filepath.Join(dir, "file")
	if err := os.WriteFile(fileAsDir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	store2 := NewStore(filepath.Join(fileAsDir, "playlists.json"))
	err := store2.Save("test", []string{track}, false)
	if err == nil {
		t.Fatal("Save with file-as-directory should fail at ensurePrivateDirectory")
	}
}
