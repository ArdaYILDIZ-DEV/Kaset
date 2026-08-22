//go:build linux

package config

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecoveryErrorFormattingAndUnwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := &RecoveryError{BackupPath: "/tmp/backup", Cause: cause}
	if !strings.Contains(err.Error(), "/tmp/backup") || !strings.Contains(err.Error(), "root cause") {
		t.Fatalf("RecoveryError.Error() = %q", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Fatal("Unwrap via errors.Is failed")
	}
	var target *RecoveryError
	if !errors.As(err, &target) {
		t.Fatal("errors.As failed for RecoveryError")
	}
}

func TestValidateBranches(t *testing.T) {
	if err := validate(Settings{Version: 99, Volume: 50}); err == nil {
		t.Fatal("unsupported version should be rejected")
	}
	if err := validate(Settings{Version: fileVersion, Volume: 50, Library: "relative/path"}); err == nil {
		t.Fatal("relative Library should be rejected")
	}
	for _, v := range []float64{math.Inf(1), -1, 101, math.NaN()} {
		if err := validate(Settings{Version: fileVersion, Volume: v}); err == nil {
			t.Fatalf("invalid volume %v should be rejected", v)
		}
	}
	if err := validate(Settings{Version: 0, Volume: 50}); err != nil {
		t.Fatalf("Version 0 should be accepted, err=%v", err)
	}
	if err := validate(Settings{Version: fileVersion, Volume: 50}); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
	if err := validate(Settings{Version: fileVersion, Volume: math.NaN()}); err == nil {
		t.Fatal("NaN volume should be rejected")
	}
	if err := validate(Settings{Version: fileVersion, Volume: math.Inf(1)}); err == nil {
		t.Fatal("Inf volume should be rejected")
	}
}

func TestSaveClampsAndCleansLibrary(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "settings.json"))
	// clamp lower bound
	if err := store.Save(Settings{Volume: -20}); err != nil {
		t.Fatalf("Save with negative volume err=%v", err)
	}
	settings, _ := store.Load()
	if settings.Volume != 0 {
		t.Fatalf("clamped lower Volume = %v, want 0", settings.Volume)
	}
	// clamp upper bound
	if err := store.Save(Settings{Volume: 200}); err != nil {
		t.Fatalf("Save with large volume err=%v", err)
	}
	settings, _ = store.Load()
	if settings.Volume != 100 {
		t.Fatalf("clamped upper Volume = %v, want 100", settings.Volume)
	}
	// Library is made absolute and cleaned
	raw := filepath.Join(dir, "a", "..", "b", "music") + "/"
	if err := store.Save(Settings{Library: raw, Volume: 50}); err != nil {
		t.Fatalf("Save Library err=%v", err)
	}
	settings, _ = store.Load()
	if !filepath.IsAbs(settings.Library) || strings.Contains(settings.Library, "..") {
		t.Fatalf("Library not absolute/clean: %q", settings.Library)
	}
	// NaN volume is rejected after clamping (clamp does not fix NaN)
	if err := store.Save(Settings{Volume: math.NaN()}); err == nil {
		t.Fatal("Save with NaN volume should fail")
	}
}

func TestLoadRecoversCorruptJSONAndInvalidVolume(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config", "settings.json")
	store := NewStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	// corrupt JSON
	if err := os.WriteFile(path, []byte("{ broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := store.Load()
	var rec *RecoveryError
	if !errors.As(err, &rec) {
		t.Fatalf("corrupt JSON should return RecoveryError, got %v", err)
	}
	if _, err := os.Stat(rec.BackupPath); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	// invalid volume
	if err := os.WriteFile(path, []byte(`{"version":1,"volume":999}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = store.Load()
	if !errors.As(err, &rec) {
		t.Fatalf("invalid volume should return RecoveryError, got %v", err)
	}
	// after recovery Load returns defaults
	settings, err := store.Load()
	if err != nil {
		t.Fatalf("Load after recovery err=%v", err)
	}
	if settings.Volume != 100 {
		t.Fatalf("defaults Volume after recovery = %v", settings.Volume)
	}
}

func TestSaveRecoversPermissionAndLockErrors(t *testing.T) {
	// Directory permission is fixed to 0700 on Save
	dir := t.TempDir()
	storeDir := filepath.Join(dir, "kaset")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store := NewStore(filepath.Join(storeDir, "settings.json"))
	if err := store.Save(Settings{Volume: 55}); err != nil {
		t.Fatalf("Save err=%v", err)
	}
	info, err := os.Stat(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("permission not fixed: %v", info.Mode().Perm())
	}
}

func TestDefaultStoreAndFileIsFree(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	store, err := DefaultStore()
	if err != nil {
		t.Fatalf("DefaultStore err=%v", err)
	}
	if !strings.HasSuffix(store.Path(), "kaset/settings.json") {
		t.Fatalf("DefaultStore path %q", store.Path())
	}
	if !fileIsFree(filepath.Join(tmp, "missing-file")) {
		t.Fatal("fileIsFree should be true for missing file")
	}
	if err := os.WriteFile(filepath.Join(tmp, "exists"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if fileIsFree(filepath.Join(tmp, "exists")) {
		t.Fatal("fileIsFree should be false for existing file")
	}
}

func TestSaveBackupCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	store := NewStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := store.Load()
	var rec1 *RecoveryError
	if !errors.As(err, &rec1) {
		t.Fatal("first recovery expected")
	}
	if err := os.WriteFile(path, []byte(`{"version":99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = store.Load()
	var rec2 *RecoveryError
	if !errors.As(err, &rec2) {
		t.Fatalf("second recovery expected %v", err)
	}
	if rec1.BackupPath == rec2.BackupPath {
		t.Fatalf("colliding backup path should differ %q", rec1.BackupPath)
	}
}
