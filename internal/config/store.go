//go:build linux

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

const fileVersion = 1

// Settings contains small pieces of state restored between sessions.
type Settings struct {
	Version       int     `json:"version"`
	Library       string  `json:"library,omitempty"`
	Volume        float64 `json:"volume"`
	ShowFolders   bool    `json:"show_folders"`
	HideSidePanel bool    `json:"hide_side_panel"`
}

// RecoveryError reports that invalid settings were moved to a backup file.
type RecoveryError struct {
	BackupPath string
	Cause      error
}

func (e *RecoveryError) Error() string {
	return fmt.Sprintf("ayar dosyası geçersizdi ve %s konumuna yedeklendi: %v", e.BackupPath, e.Cause)
}

func (e *RecoveryError) Unwrap() error {
	return e.Cause
}

// Store persists settings in the user's configuration directory.
type Store struct {
	path string
}

// DefaultStore returns the standard KASET settings store.
func DefaultStore() (*Store, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("kullanıcı yapılandırma klasörü bulunamadı: %w", err)
	}
	return NewStore(filepath.Join(configDir, "kaset", "settings.json")), nil
}

// NewStore creates a settings store backed by path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path returns the JSON file used by the store.
func (s *Store) Path() string {
	return s.path
}

// Defaults returns settings used when no persisted file exists.
func Defaults() Settings {
	return Settings{Version: fileVersion, Volume: 100}
}

// Load reads and validates persisted settings.
func (s *Store) Load() (Settings, error) {
	var settings Settings
	err := s.withLock(func() error {
		content, err := os.ReadFile(s.path)
		if errors.Is(err, os.ErrNotExist) {
			settings = Defaults()
			return nil
		}
		if err != nil {
			return fmt.Errorf("ayar dosyası okunamadı: %w", err)
		}
		if err := json.Unmarshal(content, &settings); err != nil {
			return s.recoverInvalid(fmt.Errorf("JSON çözümlenemedi: %w", err))
		}
		if err := validate(settings); err != nil {
			return s.recoverInvalid(err)
		}
		return nil
	})
	if err != nil {
		return Defaults(), err
	}
	return settings, nil
}

// Save atomically persists validated settings.
func (s *Store) Save(settings Settings) error {
	settings.Version = fileVersion
	if settings.Library != "" {
		absolute, err := filepath.Abs(settings.Library)
		if err != nil {
			return fmt.Errorf("müzik klasörü çözümlenemedi: %w", err)
		}
		settings.Library = filepath.Clean(absolute)
	}
	settings.Volume = math.Max(0, math.Min(100, settings.Volume))
	if err := validate(settings); err != nil {
		return err
	}

	return s.withLock(func() error {
		directory := filepath.Dir(s.path)
		temporary, err := os.CreateTemp(directory, ".settings-*.tmp")
		if err != nil {
			return fmt.Errorf("geçici ayar dosyası oluşturulamadı: %w", err)
		}
		temporaryPath := temporary.Name()
		defer os.Remove(temporaryPath)

		encoder := json.NewEncoder(temporary)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(settings); err != nil {
			_ = temporary.Close()
			return fmt.Errorf("ayarlar kodlanamadı: %w", err)
		}
		if err := temporary.Sync(); err != nil {
			_ = temporary.Close()
			return fmt.Errorf("ayarlar diske yazılamadı: %w", err)
		}
		if err := temporary.Close(); err != nil {
			return fmt.Errorf("ayar dosyası kapatılamadı: %w", err)
		}
		if err := os.Rename(temporaryPath, s.path); err != nil {
			return fmt.Errorf("ayar dosyası değiştirilemedi: %w", err)
		}
		return nil
	})
}

// withLock serializes access across processes and enforces private directory permissions.
func (s *Store) withLock(action func() error) error {
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("ayar klasörü oluşturulamadı: %w", err)
	}
	if info, err := os.Stat(directory); err == nil {
		if info.Mode().Perm() != 0o700 {
			if err := os.Chmod(directory, 0o700); err != nil {
				return fmt.Errorf("ayar klasörü izinleri ayarlanamadı: %w", err)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("ayar klasörü doğrulanamadı: %w", err)
	}
	lock, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("ayar kilidi açılamadı: %w", err)
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("ayar dosyası kilitlenemedi: %w", err)
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	return action()
}

// recoverInvalid preserves corrupt settings before the caller falls back to defaults.
func (s *Store) recoverInvalid(cause error) error {
	backupPath := fmt.Sprintf("%s.corrupt-%s", s.path, time.Now().Format("20060102-150405.000000000"))
	// Two corruptions in the same nanosecond would otherwise overwrite each other;
	// fall back to numbered suffixes until a free name is found.
	for attempt := 1; !fileIsFree(backupPath) && attempt <= 1000; attempt++ {
		backupPath = fmt.Sprintf("%s-%d", backupPath, attempt)
	}
	if err := os.Rename(s.path, backupPath); err != nil {
		return fmt.Errorf("geçersiz ayar dosyası yedeklenemedi: %v; asıl hata: %w", err, cause)
	}
	return &RecoveryError{BackupPath: backupPath, Cause: cause}
}

// fileIsFree reports whether path does not yet exist on disk.
func fileIsFree(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, os.ErrNotExist)
}

// validate rejects settings that cannot be safely restored in a later session.
func validate(settings Settings) error {
	// Version 0 is an unversioned (legacy) file written before versioning existed.
	// The schema is identical to the current one, so it is accepted instead of
	// being moved aside as corrupt; a future incompatible format will still be
	// rejected by the version mismatch below.
	if settings.Version != fileVersion && settings.Version != 0 {
		return fmt.Errorf("desteklenmeyen ayar dosyası sürümü: %d", settings.Version)
	}
	if settings.Library != "" && !filepath.IsAbs(settings.Library) {
		return errors.New("kayıtlı müzik klasörü mutlak değil")
	}
	if math.IsNaN(settings.Volume) || math.IsInf(settings.Volume, 0) || settings.Volume < 0 || settings.Volume > 100 {
		return fmt.Errorf("geçersiz ses seviyesi: %v", settings.Volume)
	}
	return nil
}
