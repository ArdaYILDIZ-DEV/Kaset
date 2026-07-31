package library

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Track is a local audio file shown in the library.
type Track struct {
	Path string
	Name string
}

var supportedExtensions = map[string]struct{}{
	".aac":  {},
	".ape":  {},
	".flac": {},
	".m4a":  {},
	".mp3":  {},
	".ogg":  {},
	".opus": {},
	".wav":  {},
	".wma":  {},
}

// Scan recursively finds supported audio files under root.
func Scan(root string) ([]Track, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("müzik klasörü çözümlenemedi: %w", err)
	}

	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return nil, fmt.Errorf("müzik klasörü açılamadı: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("müzik yolu bir klasör değil: %s", absoluteRoot)
	}

	tracks := make([]Track, 0)
	err = filepath.WalkDir(absoluteRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if _, ok := supportedExtensions[strings.ToLower(filepath.Ext(entry.Name()))]; !ok {
			return nil
		}

		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		tracks = append(tracks, Track{Path: path, Name: name})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("müzik klasörü taranamadı: %w", err)
	}

	sort.Slice(tracks, func(i, j int) bool {
		return strings.ToLower(tracks[i].Path) < strings.ToLower(tracks[j].Path)
	})
	return tracks, nil
}
