package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"kaset/internal/playlist"
)

func (m Model) beginPlaylistSave() (tea.Model, tea.Cmd) {
	if m.queue.Len() == 0 {
		m.setError("Kaydedilecek çalma sırası boş")
		return m, nil
	}
	if m.playlistStore == nil {
		m.setError("Çalma listesi deposu kullanılamıyor")
		return m, nil
	}
	m.libraryVisible = true
	m.prompt = promptPlaylistName
	m.pendingName = ""
	m.playlistName.SetValue(m.loadedPlaylist)
	m.playlistName.SetCursor(len([]rune(m.playlistName.Value())))
	m.clearFeedback()
	return m, m.playlistName.Focus()
}

func (m Model) handlePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.prompt {
	case promptPlaylistName:
		switch msg.String() {
		case "esc":
			m.cancelPrompt()
			return m, nil
		case "enter":
			m.submitPlaylistName()
			return m, nil
		default:
			return m.updatePlaylistName(msg)
		}
	case promptOverwrite:
		switch msg.String() {
		case "esc":
			m.cancelPrompt()
		case "enter":
			m.savePlaylist(m.pendingName, true)
		}
	case promptDeletePlaylist:
		switch msg.String() {
		case "esc":
			m.cancelPrompt()
		case "enter":
			m.deletePendingPlaylist()
		}
	}
	return m, nil
}

func (m Model) updatePlaylistName(message tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.playlistName, cmd = m.playlistName.Update(message)
	return m, cmd
}

func (m *Model) submitPlaylistName() {
	name := strings.TrimSpace(m.playlistName.Value())
	err := m.playlistStore.Save(name, m.queuePaths(), false)
	if errors.Is(err, playlist.ErrExists) {
		m.pendingName = name
		m.prompt = promptOverwrite
		m.playlistName.Blur()
		m.setNotice(fmt.Sprintf("%q zaten var; üzerine yazmak için Enter", name))
		return
	}
	if err != nil {
		m.setError(err.Error())
		return
	}
	m.finishPlaylistSave(name)
}

func (m *Model) savePlaylist(name string, overwrite bool) {
	if err := m.playlistStore.Save(name, m.queuePaths(), overwrite); err != nil {
		m.setError(err.Error())
		return
	}
	m.finishPlaylistSave(name)
}

func (m *Model) finishPlaylistSave(name string) {
	m.prompt = promptNone
	m.pendingName = ""
	m.playlistName.Blur()
	m.loadedPlaylist = name
	if !m.refreshPlaylists() {
		return
	}
	m.setNotice(fmt.Sprintf("Çalma listesi kaydedildi: %s", name))
}

func (m *Model) cancelPrompt() {
	cancelled := m.prompt
	m.prompt = promptNone
	m.pendingName = ""
	m.playlistName.Blur()
	if cancelled == promptDeletePlaylist {
		m.setNotice("Çalma listesi silme işlemi iptal edildi")
	} else {
		m.setNotice("Çalma listesi kaydı iptal edildi")
	}
}

func (m *Model) beginPlaylistDelete() {
	if m.playlistStore == nil {
		m.setError("Çalma listesi deposu kullanılamıyor")
		return
	}
	if m.playlistCursor < 0 || m.playlistCursor >= len(m.playlists) {
		m.setError("Silinecek çalma listesi yok")
		return
	}
	m.pendingName = m.playlists[m.playlistCursor].Name
	m.prompt = promptDeletePlaylist
	m.clearFeedback()
}

func (m *Model) deletePendingPlaylist() {
	name := m.pendingName
	if err := m.playlistStore.Delete(name); err != nil {
		m.setError(err.Error())
		return
	}
	m.prompt = promptNone
	m.pendingName = ""
	if m.loadedPlaylist == name {
		m.loadedPlaylist = ""
	}
	if !m.refreshPlaylists() {
		return
	}
	m.setNotice(fmt.Sprintf("Çalma listesi silindi: %s", name))
}

func (m Model) queuePaths() []string {
	items := m.queue.Items()
	paths := make([]string, 0, len(items))
	for _, trackIndex := range items {
		if trackIndex >= 0 && trackIndex < len(m.tracks) {
			paths = append(paths, m.tracks[trackIndex].Path)
		}
	}
	return paths
}

func (m *Model) openPlaylists() {
	if m.playlistStore == nil {
		m.setError("Çalma listesi deposu kullanılamıyor")
		return
	}
	m.returnPanel = m.panel
	m.clearFeedback()
	if !m.refreshPlaylists() {
		return
	}
	m.panel = panelPlaylists
	m.libraryVisible = true
	m.playlistCursor = clampCursor(m.playlistCursor, len(m.playlists))
}

func (m *Model) closePlaylists() {
	m.panel = m.returnPanel
	if m.panel == panelPlaylists {
		m.panel = panelLibrary
	}
}

func (m *Model) refreshPlaylists() bool {
	items, err := m.playlistStore.List()
	if err != nil {
		var recovery *playlist.RecoveryError
		if !errors.As(err, &recovery) {
			m.setError(err.Error())
			return false
		}
		recoveryNotice := err.Error()
		items, err = m.playlistStore.List()
		if err != nil {
			m.setError(err.Error())
			return false
		}
		m.setNotice(recoveryNotice)
	}
	m.playlists = items
	m.playlistCursor = clampCursor(m.playlistCursor, len(items))
	return true
}

func (m *Model) loadSelectedPlaylist() {
	if m.playlistCursor < 0 || m.playlistCursor >= len(m.playlists) {
		m.setError("Yüklenecek çalma listesi yok")
		return
	}
	selected := m.playlists[m.playlistCursor]
	trackByPath := make(map[string]int, len(m.tracks))
	for index, track := range m.tracks {
		trackByPath[filepath.Clean(track.Path)] = index
	}

	items := make([]int, 0, len(selected.Tracks))
	missing := 0
	for _, path := range selected.Tracks {
		trackIndex, ok := trackByPath[filepath.Clean(path)]
		if !ok {
			missing++
			continue
		}
		items = append(items, trackIndex)
	}
	if len(items) == 0 {
		m.setError("Çalma listesinde kütüphanede bulunan parça yok")
		return
	}

	m.queue.Replace(items, -1)
	m.queueCursor = 0
	m.loadedPlaylist = selected.Name
	m.panel = panelQueue
	if missing > 0 {
		m.setNotice(fmt.Sprintf("%s yüklendi; %d eksik dosya atlandı", selected.Name, missing))
	} else {
		m.setNotice(fmt.Sprintf("Çalma listesi yüklendi: %s", selected.Name))
	}
}
