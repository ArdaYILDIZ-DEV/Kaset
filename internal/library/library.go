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
	Path   string
	Name   string
	Folder string
}

// ScanIssue describes a path that could not be visited during a partial scan.
type ScanIssue struct {
	Path string
	Err  error
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

// ScanWithIssues returns supported tracks and non-fatal traversal errors.
func ScanWithIssues(root string) ([]Track, []ScanIssue, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, fmt.Errorf("müzik klasörü çözümlenemedi: %w", err)
	}

	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("müzik klasörü açılamadı: %w", err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("müzik yolu bir klasör değil: %s", absoluteRoot)
	}

	tracks := make([]Track, 0)
	issues := make([]ScanIssue, 0)
	err = filepath.WalkDir(absoluteRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		// Errors below the root are non-fatal so one unreadable folder does not hide the rest.
		if walkErr != nil {
			if path == absoluteRoot {
				return walkErr
			}
			issues = append(issues, ScanIssue{Path: path, Err: walkErr})
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if _, ok := supportedExtensions[strings.ToLower(filepath.Ext(entry.Name()))]; !ok {
			return nil
		}

		name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		folder := filepath.Dir(path)
		if relative, relativeErr := filepath.Rel(absoluteRoot, folder); relativeErr == nil && relative != "." {
			folder = relative
		} else {
			folder = ""
		}
		tracks = append(tracks, Track{Path: path, Name: name, Folder: folder})
		return nil
	})
	if err != nil {
		return nil, issues, fmt.Errorf("müzik klasörü taranamadı: %w", err)
	}

	sort.Slice(tracks, func(i, j int) bool {
		return strings.ToLower(tracks[i].Path) < strings.ToLower(tracks[j].Path)
	})
	return tracks, issues, nil
}
