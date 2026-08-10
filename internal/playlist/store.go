package playlist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

const fileVersion = 1

var (
	// ErrExists prevents an accidental overwrite unless it was explicitly confirmed.
	ErrExists = errors.New("çalma listesi zaten var")
	// ErrNotFound reports that the requested playlist is no longer stored.
	ErrNotFound = errors.New("çalma listesi bulunamadı")
)

// RecoveryError reports that invalid data was preserved under a backup path.
type RecoveryError struct {
	BackupPath string
	Cause      error
}

func (e *RecoveryError) Error() string {
	return fmt.Sprintf("çalma listesi dosyası geçersizdi ve %s konumuna yedeklendi: %v", e.BackupPath, e.Cause)
}

func (e *RecoveryError) Unwrap() error {
	return e.Cause
}

// Playlist stores tracks as direct file paths in playback order.
type Playlist struct {
	Name   string   `json:"name"`
	Tracks []string `json:"tracks"`
}

type fileData struct {
	Version   int        `json:"version"`
	Playlists []Playlist `json:"playlists"`
}

// Store persists named playlists in one atomically replaced JSON file.
type Store struct {
	path string
}

// DefaultStore uses $XDG_CONFIG_HOME/kaset/playlists.json, falling back to the
// platform's standard user configuration directory.
func DefaultStore() (*Store, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("kullanıcı yapılandırma klasörü bulunamadı: %w", err)
	}
	return NewStore(filepath.Join(configDir, "kaset", "playlists.json")), nil
}

// NewStore creates a store backed by path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path returns the JSON file used by the store.
func (s *Store) Path() string {
	return s.path
}

// List returns all playlists sorted by name.
func (s *Store) List() ([]Playlist, error) {
	var playlists []Playlist
	err := s.withLock(func() error {
		data, err := s.read()
		if err != nil {
			return err
		}
		playlists = clonePlaylists(data.Playlists)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(playlists, func(i, j int) bool {
		return strings.ToLower(playlists[i].Name) < strings.ToLower(playlists[j].Name)
	})
	return playlists, nil
}

// Load returns one named playlist.
func (s *Store) Load(name string) (Playlist, error) {
	name, err := validateName(name)
	if err != nil {
		return Playlist{}, err
	}
	var found Playlist
	err = s.withLock(func() error {
		data, err := s.read()
		if err != nil {
			return err
		}
		for _, item := range data.Playlists {
			if sameName(item.Name, name) {
				found = clonePlaylist(item)
				return nil
			}
		}
		return ErrNotFound
	})
	return found, err
}

// Save creates or updates a playlist. Existing playlists are changed only when
// overwrite is true.
func (s *Store) Save(name string, tracks []string, overwrite bool) error {
	name, err := validateName(name)
	if err != nil {
		return err
	}
	absoluteTracks, err := validateTracks(tracks, false)
	if err != nil {
		return err
	}

	return s.withLock(func() error {
		data, err := s.read()
		if err != nil {
			return err
		}
		found := -1
		for index, item := range data.Playlists {
			if sameName(item.Name, name) {
				found = index
				break
			}
		}
		updated := Playlist{Name: name, Tracks: absoluteTracks}
		switch {
		case found >= 0 && !overwrite:
			return ErrExists
		case found >= 0:
			data.Playlists[found] = updated
		default:
			data.Playlists = append(data.Playlists, updated)
		}
		return s.write(data)
	})
}

// Delete permanently removes one named playlist.
func (s *Store) Delete(name string) error {
	name, err := validateName(name)
	if err != nil {
		return err
	}
	return s.withLock(func() error {
		data, err := s.read()
		if err != nil {
			return err
		}
		for index, item := range data.Playlists {
			if !sameName(item.Name, name) {
				continue
			}
			data.Playlists = append(data.Playlists[:index], data.Playlists[index+1:]...)
			return s.write(data)
		}
		return ErrNotFound
	})
}

// withLock serializes read-modify-write operations across KASET processes.
func (s *Store) withLock(action func() error) error {
	directory := filepath.Dir(s.path)
	if err := ensurePrivateDirectory(directory); err != nil {
		return err
	}
	lock, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("çalma listesi kilidi açılamadı: %w", err)
	}
	defer lock.Close()

	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return fmt.Errorf("çalma listesi kilitlenemedi: %w", err)
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	return action()
}

// read validates persisted data and recovers corrupt files before returning.
func (s *Store) read() (fileData, error) {
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return emptyFileData(), nil
	}
	if err != nil {
		return fileData{}, fmt.Errorf("çalma listesi dosyası okunamadı: %w", err)
	}

	var data fileData
	if err := json.Unmarshal(content, &data); err != nil {
		return fileData{}, s.recoverInvalid(fmt.Errorf("JSON çözümlenemedi: %w", err))
	}
	if err := validateFileData(data); err != nil {
		return fileData{}, s.recoverInvalid(err)
	}
	return data, nil
}

// recoverInvalid moves corrupt data aside instead of overwriting it.
func (s *Store) recoverInvalid(cause error) error {
	backupPath := fmt.Sprintf("%s.corrupt-%s", s.path, time.Now().Format("20060102-150405.000000000"))
	if err := os.Rename(s.path, backupPath); err != nil {
		return fmt.Errorf("geçersiz çalma listesi dosyası yedeklenemedi: %v; asıl hata: %w", err, cause)
	}
	return &RecoveryError{BackupPath: backupPath, Cause: cause}
}

// write uses a synced temporary file and rename to avoid partial JSON updates.
func (s *Store) write(data fileData) error {
	directory := filepath.Dir(s.path)
	if err := ensurePrivateDirectory(directory); err != nil {
		return err
	}

	temporary, err := os.CreateTemp(directory, ".playlists-*.tmp")
	if err != nil {
		return fmt.Errorf("geçici çalma listesi dosyası oluşturulamadı: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("çalma listesi kodlanamadı: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("çalma listesi diske yazılamadı: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("çalma listesi dosyası kapatılamadı: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("çalma listesi dosyası değiştirilemedi: %w", err)
	}
	return nil
}

func ensurePrivateDirectory(directory string) error {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("çalma listesi klasörü oluşturulamadı: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return fmt.Errorf("çalma listesi klasörü izinleri ayarlanamadı: %w", err)
	}
	return nil
}

func emptyFileData() fileData {
	return fileData{Version: fileVersion, Playlists: []Playlist{}}
}

// validateFileData checks the on-disk schema, names, and absolute track paths.
func validateFileData(data fileData) error {
	if data.Version != fileVersion {
		return fmt.Errorf("desteklenmeyen dosya sürümü: %d", data.Version)
	}
	seen := make([]string, 0, len(data.Playlists))
	for _, item := range data.Playlists {
		name, err := validateName(item.Name)
		if err != nil {
			return fmt.Errorf("geçersiz kayıtlı çalma listesi: %w", err)
		}
		for _, existing := range seen {
			if sameName(existing, name) {
				return fmt.Errorf("yinelenen çalma listesi adı: %s", name)
			}
		}
		seen = append(seen, name)
		if _, err := validateTracks(item.Tracks, true); err != nil {
			return fmt.Errorf("%s çalma listesi geçersiz: %w", name, err)
		}
	}
	return nil
}

// validateTracks normalizes paths for writes and rejects relative paths read from disk.
func validateTracks(tracks []string, requireAbsolute bool) ([]string, error) {
	if len(tracks) == 0 {
		return nil, errors.New("boş çalma sırası kaydedilemez")
	}
	validated := make([]string, 0, len(tracks))
	for _, track := range tracks {
		if strings.TrimSpace(track) == "" {
			return nil, errors.New("çalma listesi boş dosya yolu içeremez")
		}
		cleaned := filepath.Clean(track)
		if requireAbsolute && !filepath.IsAbs(cleaned) {
			return nil, fmt.Errorf("parça yolu mutlak değil: %s", track)
		}
		if !requireAbsolute {
			absolute, err := filepath.Abs(cleaned)
			if err != nil {
				return nil, fmt.Errorf("parça yolu çözümlenemedi: %w", err)
			}
			cleaned = filepath.Clean(absolute)
		}
		validated = append(validated, cleaned)
	}
	return validated, nil
}

func sameName(left, right string) bool {
	return strings.EqualFold(left, right)
}

func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("çalma listesi adı boş olamaz")
	}
	if utf8.RuneCountInString(name) > 80 {
		return "", errors.New("çalma listesi adı 80 karakterden uzun olamaz")
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return "", errors.New("çalma listesi adı kontrol karakteri içeremez")
		}
	}
	return name, nil
}

func clonePlaylist(item Playlist) Playlist {
	return Playlist{Name: item.Name, Tracks: append([]string(nil), item.Tracks...)}
}

func clonePlaylists(items []Playlist) []Playlist {
	result := make([]Playlist, len(items))
	for index, item := range items {
		result[index] = clonePlaylist(item)
	}
	return result
}
