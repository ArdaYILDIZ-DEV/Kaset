package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"kaset/internal/library"
	"kaset/internal/player"
	"kaset/internal/playlist"
	playqueue "kaset/internal/queue"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const appName = "KASET"

type panelKind uint8

const (
	panelLibrary panelKind = iota
	panelQueue
	panelPlaylists
)

type promptMode uint8

const (
	promptNone promptMode = iota
	promptPlaylistName
	promptOverwrite
	promptDeletePlaylist
)

type playbackController interface {
	Events() <-chan player.Event
	Load(path string) error
	TogglePause() error
	Stop() error
	Seek(deltaSeconds float64) error
	ChangeVolume(delta float64) error
	ToggleMute() error
}

var (
	accentStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A78BFA"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#71717A"))
	activeStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#34D399"))
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F4F4F5"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FB7185"))
	noticeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#67E8F9"))
	playingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#C4B5FD"))
)

// Model is the Bubble Tea application state.
type Model struct {
	tracks         []library.Track
	filtered       []int
	player         playbackController
	playlistStore  *playlist.Store
	playlists      []playlist.Playlist
	queue          playqueue.Queue
	cursor         int
	queueCursor    int
	playlistCursor int
	current        int
	width          int
	height         int
	title          string
	artist         string
	album          string
	position       float64
	duration       float64
	volume         float64
	paused         bool
	muted          bool
	loopCurrent    bool
	libraryVisible bool
	panel          panelKind
	returnPanel    panelKind
	searching      bool
	search         textinput.Model
	prompt         promptMode
	playlistName   textinput.Model
	pendingName    string
	loadedPlaylist string
	errText        string
	noticeText     string
	progress       progress.Model
}

type playerEventMsg player.Event
type playerClosedMsg struct{}

// New creates the terminal UI model. A playlist store is optional so focused
// model tests can run without touching the user's configuration directory.
func New(tracks []library.Track, mpv playbackController, stores ...*playlist.Store) Model {
	bar := progress.New(
		progress.WithDefaultGradient(),
		progress.WithoutPercentage(),
		progress.WithWidth(60),
	)
	search := textinput.New()
	search.Prompt = "/ "
	search.Placeholder = "Kütüphanede ara"
	search.CharLimit = 120
	search.PromptStyle = accentStyle
	search.TextStyle = cursorStyle
	search.PlaceholderStyle = dimStyle

	playlistName := textinput.New()
	playlistName.Prompt = "> "
	playlistName.Placeholder = "Playlist adı"
	playlistName.CharLimit = 80
	playlistName.PromptStyle = accentStyle
	playlistName.TextStyle = cursorStyle
	playlistName.PlaceholderStyle = dimStyle

	filtered := make([]int, len(tracks))
	for i := range tracks {
		filtered[i] = i
	}

	var store *playlist.Store
	if len(stores) > 0 {
		store = stores[0]
	}
	return Model{
		tracks:         tracks,
		filtered:       filtered,
		player:         mpv,
		playlistStore:  store,
		queue:          playqueue.New(),
		current:        -1,
		width:          80,
		height:         24,
		volume:         100,
		paused:         true,
		libraryVisible: true,
		panel:          panelLibrary,
		returnPanel:    panelLibrary,
		search:         search,
		playlistName:   playlistName,
		progress:       bar,
	}
}

// Init starts listening for mpv events.
func (m Model) Init() tea.Cmd {
	if m.player == nil {
		return nil
	}
	return waitForPlayerEvent(m.player.Events())
}

// Update handles terminal input, queue editing, playlist prompts and mpv state changes.
func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = max(1, msg.Width)
		m.height = max(1, msg.Height)
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.prompt != promptNone {
			return m.handlePromptKey(msg)
		}
		if m.searching {
			switch msg.String() {
			case "esc", "enter":
				m.searching = false
				m.search.Blur()
				return m, nil
			}
			return m.updateSearch(msg)
		}
		return m.handleKey(msg)

	case playerEventMsg:
		m.handlePlayerEvent(player.Event(msg))
		if m.player == nil {
			return m, nil
		}
		return m, waitForPlayerEvent(m.player.Events())

	case playerClosedMsg:
		m.player = nil
		m.setError("mpv bağlantısı kapandı")
		return m, nil
	}

	if m.prompt == promptPlaylistName {
		return m.updatePlaylistName(message)
	}
	if m.searching {
		return m.updateSearch(message)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		if m.panel == panelPlaylists {
			m.closePlaylists()
		} else if m.search.Value() != "" {
			m.search.SetValue("")
			m.refreshFilter()
		}
	case "/":
		m.libraryVisible = true
		m.panel = panelLibrary
		m.searching = true
		m.search.SetCursor(len([]rune(m.search.Value())))
		return m, m.search.Focus()
	case "t":
		m.libraryVisible = !m.libraryVisible
	case "tab":
		m.libraryVisible = true
		if m.panel == panelLibrary {
			m.panel = panelQueue
		} else {
			m.panel = panelLibrary
		}
	case "P":
		if m.panel == panelPlaylists {
			m.closePlaylists()
		} else {
			m.openPlaylists()
		}
	case "S":
		return m.beginPlaylistSave()
	case " ":
		if m.current >= 0 {
			m.run(func() error { return m.player.TogglePause() })
		} else if m.queue.Len() > 0 {
			m.playQueueAt(clampCursor(m.queueCursor, m.queue.Len()))
		} else {
			m.playLibraryContext()
		}
	case "n":
		m.playNext()
	case "p":
		m.playPrevious()
	case "left", "h":
		m.run(func() error { return m.player.Seek(-5) })
	case "right":
		m.run(func() error { return m.player.Seek(5) })
	case "l":
		m.toggleLoop()
	case "+", "=", "]":
		if m.run(func() error { return m.player.ChangeVolume(5) }) {
			m.volume = min(100, m.volume+5)
		}
	case "-", "[":
		if m.run(func() error { return m.player.ChangeVolume(-5) }) {
			m.volume = max(0, m.volume-5)
		}
	case "m":
		m.run(func() error { return m.player.ToggleMute() })
	case "s":
		m.run(func() error { return m.player.Stop() })
		m.current = -1
		m.position = 0
		m.duration = 0
		m.paused = true
		m.queue.ResetPosition()
	default:
		m.handlePanelKey(msg.String())
	}
	return m, nil
}

func (m *Model) handlePanelKey(key string) {
	switch m.panel {
	case panelLibrary:
		switch key {
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
		case "down", "j":
			if len(m.filtered) > 0 {
				m.cursor = min(len(m.filtered)-1, m.cursor+1)
			}
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = max(0, len(m.filtered)-1)
		case "enter":
			m.playLibraryContext()
		case "a":
			m.appendSelectedToQueue()
		case "A":
			m.appendFilteredToQueue()
		}
	case panelQueue:
		switch key {
		case "up", "k":
			m.queueCursor = max(0, m.queueCursor-1)
		case "down", "j":
			m.queueCursor = min(max(0, m.queue.Len()-1), m.queueCursor+1)
		case "home", "g":
			m.queueCursor = 0
		case "end", "G":
			m.queueCursor = max(0, m.queue.Len()-1)
		case "enter":
			m.playQueueAt(m.queueCursor)
		case "K":
			m.moveQueueItem(-1)
		case "J":
			m.moveQueueItem(1)
		case "x":
			m.removeQueueItem()
		case "c":
			m.queue.Clear()
			m.queueCursor = 0
			m.loadedPlaylist = ""
			m.setNotice("Çalma sırası temizlendi")
		}
	case panelPlaylists:
		switch key {
		case "up", "k":
			m.playlistCursor = max(0, m.playlistCursor-1)
		case "down", "j":
			m.playlistCursor = min(max(0, len(m.playlists)-1), m.playlistCursor+1)
		case "home", "g":
			m.playlistCursor = 0
		case "end", "G":
			m.playlistCursor = max(0, len(m.playlists)-1)
		case "enter":
			m.loadSelectedPlaylist()
		case "x":
			m.beginPlaylistDelete()
		}
	}
}

func (m *Model) playLibraryContext() {
	if len(m.filtered) == 0 {
		m.setError("Arama sonucunda çalınacak parça yok")
		return
	}
	m.queue.Replace(m.filtered, -1)
	m.queueCursor = clampCursor(m.cursor, m.queue.Len())
	m.loadedPlaylist = ""
	if m.playQueueAt(m.queueCursor) {
		m.setNotice(fmt.Sprintf("%d parçalık çalma sırası oluşturuldu", m.queue.Len()))
	}
}

func (m *Model) appendSelectedToQueue() {
	trackIndex := m.selectedIndex()
	if trackIndex < 0 {
		m.setError("Kuyruğa eklenecek parça yok")
		return
	}
	wasEmpty := m.queue.Len() == 0
	m.queue.Append(trackIndex)
	if wasEmpty {
		m.queueCursor = 0
	}
	m.setNotice(fmt.Sprintf("Sıraya eklendi: %s", m.tracks[trackIndex].Name))
}

func (m *Model) appendFilteredToQueue() {
	if len(m.filtered) == 0 {
		m.setError("Kuyruğa eklenecek arama sonucu yok")
		return
	}
	wasEmpty := m.queue.Len() == 0
	m.queue.AppendMany(m.filtered)
	if wasEmpty {
		m.queueCursor = 0
	}
	m.setNotice(fmt.Sprintf("%d parça sıraya eklendi", len(m.filtered)))
}

func (m *Model) moveQueueItem(delta int) {
	to := m.queueCursor + delta
	if m.queue.Move(m.queueCursor, to) {
		m.queueCursor = to
		m.setNotice("Çalma sırası güncellendi")
	}
}

func (m *Model) removeQueueItem() {
	if _, ok := m.queue.Remove(m.queueCursor); !ok {
		return
	}
	m.queueCursor = clampCursor(m.queueCursor, m.queue.Len())
	if m.queue.Len() == 0 {
		m.loadedPlaylist = ""
	}
	m.setNotice("Parça çalma sırasından çıkarıldı")
}

func (m *Model) playQueueAt(position int) bool {
	trackIndex, ok := m.queue.ItemAt(position)
	if !ok {
		m.setError("Çalma sırasında seçili parça yok")
		return false
	}
	if !m.playTrack(trackIndex) {
		return false
	}
	_, _ = m.queue.SetPosition(position)
	m.queueCursor = position
	return true
}

func (m *Model) playTrack(trackIndex int) bool {
	if m.player == nil {
		m.setError("mpv bağlantısı kapalı")
		return false
	}
	if trackIndex < 0 || trackIndex >= len(m.tracks) {
		m.setError("Geçersiz parça seçimi")
		return false
	}
	if err := m.player.Load(m.tracks[trackIndex].Path); err != nil {
		m.setError(err.Error())
		return false
	}
	m.current = trackIndex
	m.title = m.tracks[trackIndex].Name
	m.artist = ""
	m.album = ""
	m.position = 0
	m.duration = 0
	m.paused = false
	m.clearFeedback()
	return true
}

func (m *Model) playNext() {
	position, ok := m.queue.NextPosition()
	if !ok {
		m.setNotice("Çalma sırasının sonu")
		return
	}
	m.playQueueAt(position)
}

func (m *Model) playPrevious() {
	position, ok := m.queue.PreviousPosition()
	if !ok {
		m.setNotice("Çalma sırasının başı")
		return
	}
	m.playQueueAt(position)
}

func (m *Model) toggleLoop() {
	if m.loopCurrent {
		m.loopCurrent = false
		m.setNotice("Şarkı loop kapatıldı")
		return
	}
	if m.current < 0 {
		m.setError("Loop için çalan bir şarkı yok")
		return
	}
	m.loopCurrent = true
	m.setNotice("Şarkı loop açıldı; kapatmak için l")
}

func (m Model) beginPlaylistSave() (tea.Model, tea.Cmd) {
	if m.queue.Len() == 0 {
		m.setError("Kaydedilecek çalma sırası boş")
		return m, nil
	}
	if m.playlistStore == nil {
		m.setError("Playlist deposu kullanılamıyor")
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
	m.setNotice(fmt.Sprintf("Playlist kaydedildi: %s", name))
}

func (m *Model) cancelPrompt() {
	cancelled := m.prompt
	m.prompt = promptNone
	m.pendingName = ""
	m.playlistName.Blur()
	if cancelled == promptDeletePlaylist {
		m.setNotice("Playlist silme iptal edildi")
	} else {
		m.setNotice("Playlist kaydı iptal edildi")
	}
}

func (m *Model) beginPlaylistDelete() {
	if m.playlistStore == nil {
		m.setError("Playlist deposu kullanılamıyor")
		return
	}
	if m.playlistCursor < 0 || m.playlistCursor >= len(m.playlists) {
		m.setError("Silinecek playlist yok")
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
	m.setNotice(fmt.Sprintf("Playlist silindi: %s", name))
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
		m.setError("Playlist deposu kullanılamıyor")
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
		m.setError(err.Error())
		return false
	}
	m.playlists = items
	m.playlistCursor = clampCursor(m.playlistCursor, len(items))
	return true
}

func (m *Model) loadSelectedPlaylist() {
	if m.playlistCursor < 0 || m.playlistCursor >= len(m.playlists) {
		m.setError("Yüklenecek playlist yok")
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
		m.setError("Playlistte kütüphanede bulunan parça yok")
		return
	}

	m.queue.Replace(items, -1)
	m.queueCursor = 0
	m.loadedPlaylist = selected.Name
	m.panel = panelQueue
	if missing > 0 {
		m.setNotice(fmt.Sprintf("%s yüklendi; %d eksik dosya atlandı", selected.Name, missing))
	} else {
		m.setNotice(fmt.Sprintf("Playlist yüklendi: %s", selected.Name))
	}
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
	selected := m.selectedIndex()
	query := strings.ToLower(strings.TrimSpace(m.search.Value()))
	m.filtered = m.filtered[:0]
	for index, track := range m.tracks {
		if query == "" || strings.Contains(strings.ToLower(track.Name), query) || strings.Contains(strings.ToLower(track.Path), query) {
			m.filtered = append(m.filtered, index)
		}
	}

	m.cursor = 0
	if selected < 0 {
		return
	}
	if position := indexOf(m.filtered, selected); position >= 0 {
		m.cursor = position
	}
}

func (m Model) selectedIndex() int {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return -1
	}
	return m.filtered[m.cursor]
}

func (m *Model) handlePlayerEvent(event player.Event) {
	switch event.Type {
	case player.EventProperty:
		m.handleProperty(event.Name, event.Data)
	case player.EventEndFile:
		if event.Reason != "eof" {
			return
		}
		if m.loopCurrent && m.current >= 0 {
			m.playTrack(m.current)
			return
		}
		position, ok := m.queue.NextPosition()
		if ok {
			m.playQueueAt(position)
		} else {
			m.current = -1
			m.paused = true
			m.position = m.duration
		}
	case player.EventError:
		if event.Err != nil {
			m.setError(event.Err.Error())
		}
	}
}

func (m *Model) handleProperty(name string, data any) {
	switch name {
	case "time-pos":
		m.position = numberOrZero(data)
	case "duration":
		m.duration = numberOrZero(data)
	case "pause":
		if value, ok := data.(bool); ok {
			m.paused = value
		}
	case "volume":
		m.volume = numberOrZero(data)
	case "mute":
		if value, ok := data.(bool); ok {
			m.muted = value
		}
	case "media-title":
		if value, ok := data.(string); ok && value != "" {
			m.title = value
		}
	case "metadata":
		metadata, ok := data.(map[string]any)
		if !ok {
			return
		}
		for key, value := range metadata {
			text, ok := value.(string)
			if !ok {
				continue
			}
			switch strings.ToLower(key) {
			case "title":
				if text != "" {
					m.title = text
				}
			case "artist":
				m.artist = text
			case "album":
				m.album = text
			}
		}
	}
}

func (m *Model) run(action func() error) bool {
	if m.player == nil {
		m.setError("mpv bağlantısı kapalı")
		return false
	}
	if err := action(); err != nil {
		m.setError(err.Error())
		return false
	}
	m.clearFeedback()
	return true
}

func (m *Model) setError(message string) {
	m.errText = message
	m.noticeText = ""
}

func (m *Model) setNotice(message string) {
	m.noticeText = message
	m.errText = ""
}

func (m *Model) clearFeedback() {
	m.errText = ""
	m.noticeText = ""
}

func waitForPlayerEvent(events <-chan player.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return playerClosedMsg{}
		}
		return playerEventMsg(event)
	}
}

func clampCursor(cursor, length int) int {
	if length <= 0 {
		return 0
	}
	return max(0, min(cursor, length-1))
}
