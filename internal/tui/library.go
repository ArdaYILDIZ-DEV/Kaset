package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"kaset/internal/library"
)

var (
	turkishLower = cases.Lower(language.Turkish)
	unicodeFold  = cases.Fold()
)

type libraryScanMsg struct {
	tracks []library.Track
	issues []library.ScanIssue
	err    error
}

func (m Model) beginLibraryRefresh() (tea.Model, tea.Cmd) {
	if m.scanning {
		m.setNotice("Kütüphane zaten yenileniyor")
		return m, nil
	}
	if strings.TrimSpace(m.libraryRoot) == "" {
		m.setError("Kütüphane klasörü bilinmiyor")
		return m, nil
	}
	m.scanning = true
	m.setNotice("Kütüphane yenileniyor…")
	return m, scanLibrary(m.libraryRoot)
}

func scanLibrary(root string) tea.Cmd {
	return func() tea.Msg {
		tracks, issues, err := library.ScanWithIssues(root)
		return libraryScanMsg{tracks: tracks, issues: issues, err: err}
	}
}

func (m *Model) applyLibraryScan(message libraryScanMsg) {
	m.scanning = false
	if message.err != nil {
		m.setError(message.err.Error())
		return
	}
	if len(message.tracks) == 0 {
		m.setError("Yenileme sonucunda desteklenen ses dosyası bulunamadı")
		return
	}

	selectedPath := ""
	if selected := m.selectedIndex(); selected >= 0 && selected < len(m.tracks) {
		selectedPath = filepath.Clean(m.tracks[selected].Path)
	}
	currentPath := ""
	if m.current >= 0 && m.current < len(m.tracks) {
		currentPath = filepath.Clean(m.tracks[m.current].Path)
	}

	oldItems := m.queue.Items()
	oldPosition := m.queue.Position()
	oldPaths := make([]string, 0, len(oldItems))
	for _, trackIndex := range oldItems {
		if trackIndex >= 0 && trackIndex < len(m.tracks) {
			oldPaths = append(oldPaths, filepath.Clean(m.tracks[trackIndex].Path))
		} else {
			oldPaths = append(oldPaths, "")
		}
	}

	m.tracks = message.tracks
	trackByPath := make(map[string]int, len(m.tracks))
	for index, track := range m.tracks {
		trackByPath[filepath.Clean(track.Path)] = index
	}

	remapped := make([]int, 0, len(oldPaths))
	newPosition := -1
	missing := 0
	for oldQueuePosition, path := range oldPaths {
		trackIndex, ok := trackByPath[path]
		if !ok {
			missing++
			continue
		}
		remapped = append(remapped, trackIndex)
		if oldQueuePosition == oldPosition {
			newPosition = len(remapped) - 1
		}
	}
	m.queue.Replace(remapped, newPosition)
	m.queueCursor = clampCursor(m.queueCursor, m.queue.Len())
	if missing > 0 {
		m.loadedPlaylist = ""
	}

	if currentPath != "" {
		if current, ok := trackByPath[currentPath]; ok {
			m.current = current
		} else {
			if m.player != nil {
				_ = m.player.Stop()
			}
			m.current = -1
			m.paused = true
			m.position = 0
			m.duration = 0
			m.loopCurrent = false
		}
	}

	m.refreshFilterKeeping(selectedPath)
	parts := []string{fmt.Sprintf("Kütüphane yenilendi: %d parça", len(m.tracks))}
	if len(message.issues) > 0 {
		parts = append(parts, fmt.Sprintf("%d yol okunamadı", len(message.issues)))
	}
	if missing > 0 {
		parts = append(parts, fmt.Sprintf("sıradaki %d eksik parça atlandı", missing))
	}
	m.setNotice(strings.Join(parts, " · "))
}

func (m Model) updateSearch(message tea.Msg) (tea.Model, tea.Cmd) {
	oldValue := m.search.Value()
	var cmd tea.Cmd
	m.search, cmd = m.search.Update(message)
	if m.search.Value() != oldValue {
		m.refreshFilter()
	}
	return m, cmd
}

func (m *Model) refreshFilter() {
	selectedPath := ""
	if selected := m.selectedIndex(); selected >= 0 && selected < len(m.tracks) {
		selectedPath = filepath.Clean(m.tracks[selected].Path)
	}
	m.refreshFilterKeeping(selectedPath)
}

func (m *Model) refreshFilterKeeping(selectedPath string) {
	query := strings.TrimSpace(m.search.Value())
	m.filtered = m.filtered[:0]
	for index, track := range m.tracks {
		if query == "" || searchContains(track.Name, query) || searchContains(track.Path, query) || searchContains(track.Folder, query) {
			m.filtered = append(m.filtered, index)
		}
	}

	m.cursor = 0
	if selectedPath == "" {
		return
	}
	for position, trackIndex := range m.filtered {
		if filepath.Clean(m.tracks[trackIndex].Path) == selectedPath {
			m.cursor = position
			return
		}
	}
}

func searchContains(value, query string) bool {
	return strings.Contains(turkishLower.String(value), turkishLower.String(query)) ||
		strings.Contains(unicodeFold.String(value), unicodeFold.String(query))
}

func (m Model) selectedIndex() int {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return -1
	}
	return m.filtered[m.cursor]
}
