package tui

import (
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// View renders a layout that never intentionally exceeds the current terminal
// width or height. Less important rows disappear first as the window shrinks.
func (m Model) View() string {
	width := max(1, m.width)
	height := max(1, m.height)
	if height == 1 {
		return m.startupLogo(width)
	}

	footer := dimStyle.Render(truncate(m.helpText(width), width))
	bodyLimit := height - 1
	lines := make([]string, 0, height)
	lines = append(lines, m.startupLogo(width))

	title, metadata := m.nowPlayingText()
	if height >= 3 && len(lines) < bodyLimit {
		lines = append(lines, playingStyle.Render(truncate(sanitize(title), width)))
	}
	if metadata != "" && height >= 4 && len(lines) < bodyLimit {
		lines = append(lines, dimStyle.Render(truncate(sanitize(metadata), width)))
	}
	if height >= 5 && len(lines) < bodyLimit {
		lines = append(lines, m.statusLine(width))
	}
	if height >= 6 && len(lines) < bodyLimit {
		m.progress.Width = width
		lines = append(lines, m.progress.ViewAs(m.progressPercent()))
	}

	feedback := m.feedbackLine(width)
	reserveFeedback := 0
	if feedback != "" && len(lines) < bodyLimit {
		reserveFeedback = 1
	}
	available := bodyLimit - len(lines) - reserveFeedback
	if available > 0 {
		switch {
		case m.helpVisible:
			lines = append(lines, m.helpView(width, available)...)
		case m.prompt != promptNone:
			lines = append(lines, m.promptView(width, available)...)
		case m.listsVisible:
			lines = append(lines, m.panelView(width, available)...)
		default:
			lines = append(lines, dimStyle.Render(truncate("Listeler gizli · t ile aç", width)))
		}
	}

	for len(lines) < bodyLimit-reserveFeedback {
		lines = append(lines, "")
	}
	if reserveFeedback == 1 {
		lines = append(lines, feedback)
	}
	for len(lines) < bodyLimit {
		lines = append(lines, "")
	}
	lines = append(lines, footer)
	return strings.Join(lines[:height], "\n")
}

// startupLogo sweeps the accent color from left to right across the name.
func (m Model) startupLogo(width int) string {
	name := truncate(appName, width)
	var result strings.Builder
	for index, character := range []rune(name) {
		styleIndex := startupLogoStyleIndex(m.startupFrame, index)
		result.WriteString(startupLogoStyles[styleIndex].Render(string(character)))
	}
	return result.String()
}

func startupLogoStyleIndex(frame, letter int) int {
	return min(len(startupLogoStyles)-1, max(0, 2*frame-letter))
}

// panelView chooses a split or single-panel layout based on terminal width.
func (m Model) panelView(width, available int) []string {
	if width >= 100 {
		if m.sidePanelEnabled {
			return m.wideLibraryView(width, available, m.sidePanel)
		}
		return m.libraryView(width, available)
	}
	switch m.panel {
	case panelQueue:
		return m.queueView(width, available)
	case panelPlaylists:
		return m.playlistsView(width, available)
	default:
		return m.libraryView(width, available)
	}
}

// wideLibraryView gives the library three fifths of the split layout and fills its height.
func (m Model) wideLibraryView(width, available int, rightPanel panelKind) []string {
	contentWidth := max(2, width-3)
	leftWidth := contentWidth * 3 / 5
	rightWidth := contentWidth - leftWidth
	left := m.libraryView(leftWidth, available)
	var right []string
	if rightPanel == panelPlaylists {
		right = m.playlistsView(rightWidth, available)
	} else {
		right = m.queueView(rightWidth, available)
	}
	rows := available
	lines := make([]string, 0, rows)
	divider := dividerStyle.Render(" │ ")
	for row := 0; row < rows; row++ {
		leftLine := ""
		if row < len(left) {
			leftLine = left[row]
		}
		rightLine := ""
		if row < len(right) {
			rightLine = right[row]
		}
		lines = append(lines, padRight(leftLine, leftWidth)+divider+truncate(rightLine, rightWidth))
	}
	return lines
}

func (m Model) panelHeader(title string, panel panelKind, width int) string {
	if m.width < 100 {
		return accentStyle.Render(truncate(title, width))
	}
	if m.panel == panel {
		return accentStyle.Render(truncate("● "+title, width))
	}
	return dimStyle.Render(truncate("○ "+title, width))
}

func (m Model) libraryView(width, available int) []string {
	if available <= 0 {
		return nil
	}

	count := fmt.Sprintf("KÜTÜPHANE  %d parça", len(m.tracks))
	if m.search.Value() != "" {
		count = fmt.Sprintf("KÜTÜPHANE  %d/%d parça", len(m.filtered), len(m.tracks))
	}
	lines := []string{m.panelHeader(count, panelLibrary, width)}
	available--

	if available > 0 && (m.searching || m.search.Value() != "") {
		if m.searching {
			if width < 3 {
				lines = append(lines, accentStyle.Render(truncate("/", width)))
			} else {
				search := m.search
				search.Width = max(1, width-lipgloss.Width(search.Prompt))
				if value := sanitize(search.Value()); value != search.Value() {
					search.SetValue(value)
				}
				lines = append(lines, search.View())
			}
		} else {
			query := fmt.Sprintf("/ %s  ·  Esc temizle", sanitize(m.search.Value()))
			lines = append(lines, dimStyle.Render(truncate(query, width)))
		}
		available--
	}

	if available <= 0 {
		return lines
	}
	if len(m.filtered) == 0 {
		return append(lines, dimStyle.Render(truncate("Sonuç bulunamadı", width)))
	}

	start, end := visibleRange(m.cursor, len(m.filtered), available)
	for visibleIndex := start; visibleIndex < end; visibleIndex++ {
		trackIndex := m.filtered[visibleIndex]
		cursor := "  "
		if visibleIndex == m.cursor && m.panel == panelLibrary {
			cursor = "> "
		}
		playing := "  "
		if trackIndex == m.current {
			playing = "▶ "
		}
		nameWidth := max(1, width-10)
		label := m.tracks[trackIndex].Name
		if m.showFolders && m.tracks[trackIndex].Folder != "" {
			label += " :: " + m.tracks[trackIndex].Folder
		}
		line := fmt.Sprintf("%s%s%3d  %s", cursor, playing, trackIndex+1, truncate(sanitize(label), nameWidth))
		line = truncate(line, width)
		switch {
		case trackIndex == m.current:
			line = activeStyle.Render(line)
		case visibleIndex == m.cursor && m.panel == panelLibrary:
			line = cursorStyle.Bold(true).Render(line)
		default:
			line = dimStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}

func (m Model) queueView(width, available int) []string {
	if available <= 0 {
		return nil
	}

	header := fmt.Sprintf("ÇALMA SIRASI  %d parça", m.queue.Len())
	if m.loadedPlaylist != "" {
		header = fmt.Sprintf("ÇALMA SIRASI  %s  ·  %d", m.loadedPlaylist, m.queue.Len())
	}
	lines := []string{m.panelHeader(header, panelQueue, width)}
	available--
	if available <= 0 {
		return lines
	}
	if m.queue.Len() == 0 {
		return append(lines, dimStyle.Render(truncate("Sıra boş · kütüphanede a veya A kullan", width)))
	}

	start, end := visibleRange(m.queueCursor, m.queue.Len(), available)
	for position := start; position < end; position++ {
		trackIndex, ok := m.queue.ItemAt(position)
		if !ok || trackIndex < 0 || trackIndex >= len(m.tracks) {
			continue
		}
		cursor := "  "
		if position == m.queueCursor && m.panel == panelQueue {
			cursor = "> "
		}
		playing := "  "
		if position == m.queue.Position() && trackIndex == m.current {
			playing = "▶ "
		}
		nameWidth := max(1, width-10)
		line := fmt.Sprintf("%s%s%3d  %s", cursor, playing, position+1, truncate(sanitize(m.tracks[trackIndex].Name), nameWidth))
		line = truncate(line, width)
		switch {
		case position == m.queue.Position() && trackIndex == m.current:
			line = activeStyle.Render(line)
		case position == m.queueCursor && m.panel == panelQueue:
			line = cursorStyle.Bold(true).Render(line)
		default:
			line = dimStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}

func (m Model) playlistsView(width, available int) []string {
	if available <= 0 {
		return nil
	}
	header := fmt.Sprintf("ÇALMA LİSTELERİ  %d", len(m.playlists))
	lines := []string{m.panelHeader(header, panelPlaylists, width)}
	available--
	if available <= 0 {
		return lines
	}
	if len(m.playlists) == 0 {
		return append(lines, dimStyle.Render(truncate("Kayıtlı çalma listesi yok · sırayı S ile kaydet", width)))
	}

	start, end := visibleRange(m.playlistCursor, len(m.playlists), available)
	for position := start; position < end; position++ {
		item := m.playlists[position]
		selected := position == m.playlistCursor && m.panel == panelPlaylists
		cursor := "  "
		if selected {
			cursor = "> "
		}
		line := fmt.Sprintf("%s%3d  %s  (%d)", cursor, position+1, sanitize(item.Name), len(item.Tracks))
		line = truncate(line, width)
		if selected {
			line = cursorStyle.Bold(true).Render(line)
		} else {
			line = dimStyle.Render(line)
		}
		lines = append(lines, line)
	}
	return lines
}

func (m Model) promptView(width, available int) []string {
	if available <= 0 {
		return nil
	}
	title := "ÇALMA LİSTESİ KAYDET"
	if m.prompt == promptDeletePlaylist {
		title = "ÇALMA LİSTESİ SİL"
	}
	lines := []string{accentStyle.Render(truncate(title, width))}
	available--
	if available <= 0 {
		return lines
	}

	if m.prompt == promptDeletePlaylist {
		message := fmt.Sprintf("%q kalıcı olarak silinsin mi?", m.pendingName)
		lines = append(lines, cursorStyle.Render(truncate(message, width)))
		available--
		if available > 0 {
			lines = append(lines, dimStyle.Render(truncate("Enter sil · Esc iptal", width)))
		}
		return lines
	}

	if m.prompt == promptOverwrite {
		message := fmt.Sprintf("%q zaten var. Üzerine yazılsın mı?", m.pendingName)
		lines = append(lines, cursorStyle.Render(truncate(message, width)))
		available--
		if available > 0 {
			lines = append(lines, dimStyle.Render(truncate("Enter üzerine yaz · Esc iptal", width)))
		}
		return lines
	}

	if width < 3 {
		lines = append(lines, accentStyle.Render(truncate(">", width)))
	} else {
		playlistName := m.playlistName
		playlistName.Width = max(1, width-lipgloss.Width(playlistName.Prompt))
		if value := sanitize(playlistName.Value()); value != playlistName.Value() {
			playlistName.SetValue(value)
		}
		lines = append(lines, playlistName.View())
	}
	available--
	if available > 0 {
		lines = append(lines, dimStyle.Render(truncate("Enter kaydet · Esc iptal", width)))
	}
	return lines
}

func (m Model) helpView(width, available int) []string {
	content := []string{
		"YARDIM",
		"Space oynat/duraklat   n/p sonraki/önceki   ←/→ sar",
		"l tek parça döngüsü   R sıra döngüsü   z sıradakileri karıştır",
		"+/- ses   m sessiz   s durdur",
		"j/k veya ↑/↓ gezin   g/G başa/sona git   Enter seç",
		"a seçileni ekle   A görünenleri ekle   / ara   r yenile   d klasör ayrıntısı",
		"Tab odağı görünür paneller arasında değiştirir",
		"y çalma sırası   P çalma listeleri   S kaydet",
		"J/K sırada taşı   x kaldır/sil   c sırayı temizle",
		"t tüm listeleri gizle   ?/Esc yardımı kapat   q çık",
	}
	lines := make([]string, 0, min(available, len(content)))
	for index, line := range content {
		if len(lines) >= available {
			break
		}
		style := dimStyle
		if index == 0 {
			style = accentStyle
		}
		lines = append(lines, style.Render(truncate(line, width)))
	}
	return lines
}

func (m Model) feedbackLine(width int) string {
	if m.errText != "" {
		return errorStyle.Render(truncate("Hata: "+sanitize(m.errText), width))
	}
	if m.noticeText != "" {
		return noticeStyle.Render(truncate(sanitize(m.noticeText), width))
	}
	return ""
}

func (m Model) nowPlayingText() (string, string) {
	title := m.title
	if title == "" {
		if m.current >= 0 && m.current < len(m.tracks) {
			title = m.tracks[m.current].Name
		} else {
			title = "Bir parça seç"
		}
	}
	metadata := strings.Join(nonEmpty(m.artist, m.album), "  •  ")
	return title, metadata
}

func (m Model) statusLine(width int) string {
	state := "DURAKLATILDI"
	stateStyle := dimStyle
	if m.current < 0 {
		state = "HAZIR"
	} else if !m.paused {
		state = "ÇALIYOR"
		stateStyle = activeStyle
	}
	if m.muted {
		state += " · SESSİZ"
	}
	if m.loopCurrent {
		state += " · PARÇA DÖNGÜSÜ"
	}
	if m.repeatQueue {
		state += " · SIRA DÖNGÜSÜ"
	}

	timeText := fmt.Sprintf("%s/%s", formatDuration(m.position), formatDuration(m.duration))
	queueText := ""
	if m.queue.Position() >= 0 {
		queueText = fmt.Sprintf("  %d/%d", m.queue.Position()+1, m.queue.Len())
	}
	var line string
	switch {
	case width >= 58:
		line = fmt.Sprintf("%s   %s   Ses: %s%s", state, timeText, volumeLabel(m.volume, m.muted), queueText)
	case width >= 32:
		line = fmt.Sprintf("%s  %s  V:%s", state, timeText, volumeLabel(m.volume, m.muted))
	default:
		line = fmt.Sprintf("%s %s", state, timeText)
	}
	return stateStyle.Render(truncate(line, width))
}

func (m Model) progressPercent() float64 {
	if m.duration <= 0 {
		return 0
	}
	return math.Max(0, math.Min(1, m.position/m.duration))
}

func (m Model) helpText(width int) string {
	if m.helpVisible {
		return "?/Esc yardımı kapat  q çık"
	}
	if m.prompt == promptDeletePlaylist {
		return "Enter çalma listesini sil  Esc iptal"
	}
	if m.prompt == promptOverwrite {
		return "Enter üzerine yaz  Esc iptal"
	}
	if m.prompt == promptPlaylistName {
		return "Çalma listesi adı  Enter kaydet  Esc iptal"
	}
	if m.searching {
		return "Arama yaz  Enter/Esc bitir  Esc tekrar temizle"
	}
	if !m.listsVisible {
		return "t listeleri aç  Space oynat/duraklat  n/p geçiş  ? yardım  q çık"
	}
	if width < 24 {
		return "Space n/p y P t ? q"
	}
	if width < 52 {
		return "Space n/p  y sıra  P listeler  t gizle  ? yardım  q"
	}
	if !m.sidePanelEnabled {
		return "Enter sırayı çal  a/A ekle  / ara  y sıra  P listeler  t tümünü gizle  ? yardım  q"
	}
	switch m.panel {
	case panelQueue:
		return "Enter oynat  J/K taşı  x çıkar  c temizle  y kapat  Tab → kütüphane  ? yardım  q"
	case panelPlaylists:
		return "Enter sıraya yükle  x sil  P/Esc kapat  y → sıra  Tab → kütüphane  ? yardım  q"
	default:
		if m.sidePanel == panelPlaylists {
			return "Enter sırayı çal  a/A ekle  / ara  Tab → listeler  P kapat  y → sıra  t gizle  ? yardım  q"
		}
		return "Enter sırayı çal  a/A ekle  / ara  Tab → sıra  y kapat  P listeler  t gizle  ? yardım  q"
	}
}

// visibleRange centers the cursor when possible and keeps the window in bounds.
func visibleRange(cursor, total, rows int) (int, int) {
	if rows <= 0 || total <= 0 {
		return 0, 0
	}
	if total <= rows {
		return 0, total
	}
	start := cursor - rows/2
	start = max(0, min(start, total-rows))
	return start, start + rows
}

func formatDuration(seconds float64) string {
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		seconds = 0
	}
	total := int(seconds)
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}

func volumeLabel(volume float64, muted bool) string {
	if muted {
		return "kapalı"
	}
	return fmt.Sprintf("%.0f%%", math.Max(0, math.Min(100, volume)))
}

func numberOrZero(value any) float64 {
	if number, ok := value.(float64); ok {
		return number
	}
	return 0
}

// sanitize strips control characters before user-controlled text reaches the terminal.
func sanitize(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}

// truncate limits text by terminal cell width rather than rune or byte count.
func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(value, width, "…")
}

func padRight(value string, width int) string {
	value = truncate(value, width)
	padding := max(0, width-ansi.StringWidth(value))
	return value + strings.Repeat(" ", padding)
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}
