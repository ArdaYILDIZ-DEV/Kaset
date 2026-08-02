package tui

import (
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"kaset/internal/library"
	"kaset/internal/player"
	"kaset/internal/playlist"
	playqueue "kaset/internal/queue"
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
	dividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4C4663"))
)

// Options configures optional model capabilities.
type Options struct {
	PlaylistStore *playlist.Store
	LibraryRoot   string
	InitialNotice string
	InitialVolume *float64
	ShowFolders   bool
}

// Model is the Bubble Tea application state.
type Model struct {
	tracks         []library.Track
	filtered       []int
	player         playbackController
	playlistStore  *playlist.Store
	playlists      []playlist.Playlist
	queue          playqueue.Queue
	libraryRoot    string
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
	repeatQueue    bool
	showFolders    bool
	libraryVisible bool
	helpVisible    bool
	scanning       bool
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

// New creates a model while preserving the original focused-test API.
func New(tracks []library.Track, mpv playbackController, stores ...*playlist.Store) Model {
	options := Options{}
	if len(stores) > 0 {
		options.PlaylistStore = stores[0]
	}
	return NewWithOptions(tracks, mpv, options)
}

// NewWithOptions creates a fully configured terminal UI model.
func NewWithOptions(tracks []library.Track, mpv playbackController, options Options) Model {
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
	playlistName.Placeholder = "Çalma listesi adı"
	playlistName.CharLimit = 80
	playlistName.PromptStyle = accentStyle
	playlistName.TextStyle = cursorStyle
	playlistName.PlaceholderStyle = dimStyle

	filtered := make([]int, len(tracks))
	for index := range tracks {
		filtered[index] = index
	}
	initialVolume := 100.0
	if options.InitialVolume != nil {
		initialVolume = max(0, min(100, *options.InitialVolume))
	}

	return Model{
		tracks:         tracks,
		filtered:       filtered,
		player:         mpv,
		playlistStore:  options.PlaylistStore,
		queue:          playqueue.New(),
		libraryRoot:    options.LibraryRoot,
		current:        -1,
		width:          80,
		height:         24,
		volume:         initialVolume,
		paused:         true,
		showFolders:    options.ShowFolders,
		libraryVisible: true,
		panel:          panelLibrary,
		returnPanel:    panelLibrary,
		search:         search,
		playlistName:   playlistName,
		noticeText:     options.InitialNotice,
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

// Volume returns the most recently observed playback volume.
func (m Model) Volume() float64 {
	return m.volume
}

// ShowFolders reports whether library folder details are visible.
func (m Model) ShowFolders() bool {
	return m.showFolders
}

// Update handles terminal input and asynchronous application events.
func (m Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = max(1, msg.Width)
		m.height = max(1, msg.Height)
		return m, nil
	case libraryScanMsg:
		m.applyLibraryScan(msg)
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.helpVisible {
			switch msg.String() {
			case "?", "esc":
				m.helpVisible = false
			case "q":
				return m, tea.Quit
			}
			return m, nil
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
	case "?":
		m.helpVisible = true
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
			m.setNotice("Odak: Çalma sırası")
		} else {
			m.panel = panelLibrary
			m.setNotice("Odak: Kütüphane")
		}
	case "P":
		if m.panel == panelPlaylists {
			m.closePlaylists()
		} else {
			m.openPlaylists()
		}
	case "S":
		return m.beginPlaylistSave()
	case "r":
		return m.beginLibraryRefresh()
	case "d":
		m.showFolders = !m.showFolders
		if m.showFolders {
			m.setNotice("Klasör ayrıntıları gösteriliyor")
		} else {
			m.setNotice("Klasör ayrıntıları gizlendi")
		}
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
		m.toggleCurrentLoop()
	case "R":
		m.toggleQueueRepeat()
	case "z":
		m.shuffleUpcoming()
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
		if m.run(func() error { return m.player.Stop() }) {
			m.current = -1
			m.position = 0
			m.duration = 0
			m.paused = true
			m.loopCurrent = false
			m.queue.ResetPosition()
		}
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
