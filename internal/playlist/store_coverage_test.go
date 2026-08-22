//go:build linux

package playlist

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoveryErrorFormatting(t *testing.T) {
	cause := errors.New("root cause")
	err := &RecoveryError{BackupPath: "/tmp/backup", Cause: cause}
	if !strings.Contains(err.Error(), "/tmp/backup") || !strings.Contains(err.Error(), "root cause") {
		t.Fatalf("RecoveryError.Error() = %q", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Fatal("Unwrap failed")
	}
	var target *RecoveryError
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed")
	}
}

func TestValidateNameBranches(t *testing.T) {
	if _, err := validateName("  "); err == nil {
		t.Fatal("empty name should be rejected")
	}
	long := strings.Repeat("a", 81)
	if _, err := validateName(long); err == nil {
		t.Fatal("81-char name should be rejected")
	}
	if _, err := validateName("valid\x00name"); err == nil {
		t.Fatal("control character should be rejected")
	}
	if _, err := validateName("  valid  "); err != nil {
		t.Fatalf("trimmed valid name rejected: %v", err)
	}
	exact := strings.Repeat("b", 80)
	if _, err := validateName(exact); err != nil {
		t.Fatalf("80-char name rejected: %v", err)
	}
}

func TestValidateTracksBranches(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "a.mp3")
	if _, err := validateTracks(nil, false); err == nil {
		t.Fatal("nil tracks should be rejected")
	}
	if _, err := validateTracks([]string{}, false); err == nil {
		t.Fatal("empty tracks should be rejected")
	}
	if _, err := validateTracks([]string{"  "}, false); err == nil {
		t.Fatal("blank path should be rejected")
	}
	if _, err := validateTracks([]string{"relative.mp3"}, true); err == nil {
		t.Fatal("relative path should be rejected when requireAbsolute is true")
	}
	got, err := validateTracks([]string{"relative.mp3"}, false)
	if err != nil || !filepath.IsAbs(got[0]) {
		t.Fatalf("relative path should be made absolute: %v %q", err, got)
	}
	got, err = validateTracks([]string{abs}, true)
	if err != nil {
		t.Fatalf("absolute path rejected: %v", err)
	}
	if got[0] != filepath.Clean(abs) {
		t.Fatalf("absolute path cleaning failed: %q", got[0])
	}
	got, err = validateTracks([]string{abs + "/../" + "b.mp3"}, false)
	if err != nil {
		t.Fatalf("cleaning err=%v", err)
	}
	if strings.Contains(got[0], "..") {
		t.Fatalf("path not cleaned: %q", got[0])
	}
}

func TestValidateFileDataBranches(t *testing.T) {
	dir := t.TempDir()
	track := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(track, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateFileData(fileData{Version: 99, Playlists: []Playlist{{Name: "a", Tracks: []string{track}}}}); err == nil {
		t.Fatal("unsupported version should be rejected")
	}
	if err := validateFileData(fileData{Version: 0, Playlists: []Playlist{{Name: "a", Tracks: []string{track}}}}); err != nil {
		t.Fatalf("version 0 should be accepted: %v", err)
	}
	if err := validateFileData(fileData{Version: fileVersion, Playlists: []Playlist{{Name: "  ", Tracks: []string{track}}}}); err == nil {
		t.Fatal("invalid name should be rejected")
	}
	if err := validateFileData(fileData{Version: fileVersion, Playlists: []Playlist{
		{Name: "Jazz", Tracks: []string{track}},
		{Name: "jazz", Tracks: []string{track}},
	}}); err == nil {
		t.Fatal("duplicate name should be rejected")
	}
	if err := validateFileData(fileData{Version: fileVersion, Playlists: []Playlist{{Name: "a", Tracks: []string{"relative.mp3"}}}}); err == nil {
		t.Fatal("relative track should be rejected")
	}
	if err := validateFileData(fileData{Version: fileVersion, Playlists: []Playlist{{Name: "Evening", Tracks: []string{track}}}}); err != nil {
		t.Fatalf("valid fileData rejected: %v", err)
	}
}

func TestStoreWriteAndReadErrorPaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kaset", "playlists.json")
	store := NewStore(path)
	track := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(track, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("  ", []string{track}, false); err == nil {
		t.Fatal("Save with empty name should fail")
	}
	if err := store.Save("Valid", nil, false); err == nil {
		t.Fatal("Save with empty list should fail")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{ broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := store.List()
	var rec *RecoveryError
	if !errors.As(err, &rec) {
		t.Fatalf("corrupt JSON should return RecoveryError: %v", err)
	}
	items, err := store.List()
	if err != nil || len(items) != 0 {
		t.Fatalf("after recovery List %v err=%v", items, err)
	}
	if err := os.WriteFile(path, []byte(`{"version":99,"playlists":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = store.List()
	if !errors.As(err, &rec) {
		t.Fatalf("invalid version should return RecoveryError: %v", err)
	}
	dup := fileData{Version: fileVersion, Playlists: []Playlist{{Name: "A", Tracks: []string{track}}, {Name: "a", Tracks: []string{track}}}}
	data, _ := json.Marshal(dup)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = store.List()
	if !errors.As(err, &rec) {
		t.Fatalf("duplicate name should return RecoveryError: %v", err)
	}
}

func TestEnsurePrivateDirectoryAndFileIsFree(t *testing.T) {
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "kaset")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateDirectory(storeDir); err != nil {
		t.Fatalf("ensurePrivateDirectory err=%v", err)
	}
	info, err := os.Stat(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("permission not fixed: %v", info.Mode().Perm())
	}
	if !fileIsFree(filepath.Join(dir, "missing")) {
		t.Fatal("fileIsFree should be true for missing file")
	}
	tmpFile := filepath.Join(dir, "exists")
	if err := os.WriteFile(tmpFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if fileIsFree(tmpFile) {
		t.Fatal("fileIsFree should be false for existing file")
	}
}

func TestLoadAndListAfterRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "playlists.json")
	store := NewStore(path)
	track := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(track, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("One", []string{track}, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := store.Load("One")
	var rec *RecoveryError
	if !errors.As(err, &rec) {
		t.Fatalf("Load should return RecoveryError: %v", err)
	}
	_, err = store.Load("One")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("after recovery Load should return ErrNotFound: %v", err)
	}
}

func TestDeleteAndOverwriteEdge(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "playlists.json"))
	track := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(track, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("Jazz", []string{track}, false); err != nil {
		t.Fatal(err)
	}
	if err := store.Save("jazz", []string{track}, false); !errors.Is(err, ErrExists) {
		t.Fatalf("case-variant should return ErrExists: %v", err)
	}
	if err := store.Delete("JAZZ"); err != nil {
		t.Fatalf("case-insensitive Delete err=%v", err)
	}
	if err := store.Delete("JAZZ"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second Delete should return ErrNotFound: %v", err)
	}
	if err := store.Delete("  "); err == nil {
		t.Fatal("Delete with empty name should fail")
	}
}

func TestSaveBackupCollisionPlaylist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "playlists.json")
	store := NewStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := store.List()
	var rec1 *RecoveryError
	if !errors.As(err, &rec1) {
		t.Fatal("first recovery expected")
	}
	if err := os.WriteFile(path, []byte(`{"version":99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = store.List()
	var rec2 *RecoveryError
	if !errors.As(err, &rec2) {
		t.Fatalf("second recovery expected %v", err)
	}
	if rec1.BackupPath == rec2.BackupPath {
		t.Fatalf("colliding backup should differ %q", rec1.BackupPath)
	}
}
