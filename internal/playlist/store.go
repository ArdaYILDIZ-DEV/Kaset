package playlist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const fileVersion = 1

var (
	ErrExists   = errors.New("playlist zaten var")
	ErrNotFound = errors.New("playlist bulunamadı")
)

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

// NewStore creates a store backed by path. It does not touch the filesystem
// until a read or write operation is requested.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path returns the JSON file used by the store.
func (s *Store) Path() string {
	return s.path
}

// List returns all playlists sorted by name.
func (s *Store) List() ([]Playlist, error) {
	data, err := s.read()
	if err != nil {
		return nil, err
	}
	playlists := clonePlaylists(data.Playlists)
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
	data, err := s.read()
	if err != nil {
		return Playlist{}, err
	}
	for _, item := range data.Playlists {
		if item.Name == name {
			return clonePlaylist(item), nil
		}
	}
	return Playlist{}, ErrNotFound
}

// Save creates or updates a playlist. Existing playlists are changed only when
// overwrite is true.
func (s *Store) Save(name string, tracks []string, overwrite bool) error {
	name, err := validateName(name)
	if err != nil {
		return err
	}
	if len(tracks) == 0 {
		return errors.New("boş çalma sırası kaydedilemez")
	}

	absoluteTracks := make([]string, 0, len(tracks))
	for _, track := range tracks {
		if strings.TrimSpace(track) == "" {
			return errors.New("playlist boş dosya yolu içeremez")
		}
		absolute, err := filepath.Abs(track)
		if err != nil {
			return fmt.Errorf("parça yolu çözümlenemedi: %w", err)
		}
		absoluteTracks = append(absoluteTracks, filepath.Clean(absolute))
	}

	data, err := s.read()
	if err != nil {
		return err
	}
	found := -1
	for index, item := range data.Playlists {
		if item.Name == name {
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
}

// Delete permanently removes one named playlist.
func (s *Store) Delete(name string) error {
	name, err := validateName(name)
	if err != nil {
		return err
	}
	data, err := s.read()
	if err != nil {
		return err
	}
	for index, item := range data.Playlists {
		if item.Name != name {
			continue
		}
		data.Playlists = append(data.Playlists[:index], data.Playlists[index+1:]...)
		return s.write(data)
	}
	return ErrNotFound
}

func (s *Store) read() (fileData, error) {
	content, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return fileData{Version: fileVersion, Playlists: []Playlist{}}, nil
	}
	if err != nil {
		return fileData{}, fmt.Errorf("playlist dosyası okunamadı: %w", err)
	}

	var data fileData
	if err := json.Unmarshal(content, &data); err != nil {
		return fileData{}, fmt.Errorf("playlist dosyası geçersiz: %w", err)
	}
	if data.Version != fileVersion {
		return fileData{}, fmt.Errorf("desteklenmeyen playlist dosya sürümü: %d", data.Version)
	}
	for _, item := range data.Playlists {
		if _, err := validateName(item.Name); err != nil {
			return fileData{}, fmt.Errorf("geçersiz kayıtlı playlist: %w", err)
		}
	}
	return data, nil
}

func (s *Store) write(data fileData) error {
	directory := filepath.Dir(s.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("playlist klasörü oluşturulamadı: %w", err)
	}

	temporary, err := os.CreateTemp(directory, ".playlists-*.tmp")
	if err != nil {
		return fmt.Errorf("geçici playlist dosyası oluşturulamadı: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("playlist kodlanamadı: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("playlist diske yazılamadı: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("playlist dosyası kapatılamadı: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("playlist dosyası değiştirilemedi: %w", err)
	}
	return nil
}

func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("playlist adı boş olamaz")
	}
	if utf8.RuneCountInString(name) > 80 {
		return "", errors.New("playlist adı 80 karakterden uzun olamaz")
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return "", errors.New("playlist adı kontrol karakteri içeremez")
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
