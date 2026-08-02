package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultsAndSaveRoundTrip(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "config", "settings.json"))
	settings, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Version != fileVersion || settings.Library != "" || settings.Volume != 100 {
		t.Fatalf("Load() defaults = %#v", settings)
	}

	library := t.TempDir()
	if err := store.Save(Settings{Library: library, Volume: 42}); err != nil {
		t.Fatal(err)
	}
	settings, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Library != library || settings.Volume != 42 {
		t.Fatalf("Load() = %#v", settings)
	}
}

func TestSaveClampsVolume(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	if err := store.Save(Settings{Volume: 140}); err != nil {
		t.Fatal(err)
	}
	settings, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.Volume != 100 {
		t.Fatalf("Volume = %v, want 100", settings.Volume)
	}
}

func TestLoadRecoversInvalidSettings(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "settings.json"))
	if err := os.MkdirAll(filepath.Dir(store.Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.Path(), []byte(`{"version":99,"volume":100}`), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := store.Load()
	var recovery *RecoveryError
	if !errors.As(err, &recovery) {
		t.Fatalf("Load() error = %v, want RecoveryError", err)
	}
	if settings.Volume != 100 {
		t.Fatalf("Load() fallback = %#v", settings)
	}
	if _, err := os.Stat(recovery.BackupPath); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
}

func TestDefaultStoreUsesKasetConfigPath(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	store, err := DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(configDir, "kaset", "settings.json"); store.Path() != want {
		t.Fatalf("Path() = %q, want %q", store.Path(), want)
	}
}
