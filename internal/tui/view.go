package tui

import (
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

// View renders a layout that never intentionally exceeds the current terminal
// width or height. Less important rows disappear first as the window shrinks.
func (m Model) View() string {
	width := max(1, m.width)
	height := max(1, m.height)
	if height == 1 {
		return accentStyle.Render(truncate(appName, width))
	}

	footer := dimStyle.Render(truncate(m.helpText(width), width))
	bodyLimit := height - 1
	lines := make([]string, 0, height)
	lines = append(lines, accentStyle.Render(truncate(appName, width)))

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
		case m.prompt != promptNone:
			lines = append(lines, m.promptView(width, available)...)
		case m.libraryVisible:
			lines = append(lines, m.panelView(width, available)...)
		default:
			lines = append(lines, dimStyle.Render(truncate("Panel gizli · t ile aç", width)))
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

func (m Model) panelView(width, available int) []string {
	switch m.panel {
	case panelQueue:
		return m.queueView(width, available)
	case panelPlaylists:
		return m.playlistsView(width, available)
	default:
		return m.libraryView(width, available)
	}
}

func (m Model) libraryView(width, available int) []string {
	if available <= 0 {
		return nil
	}

	count := fmt.Sprintf("KÜTÜPHANE  %d parça", len(m.tracks))
	if m.search.Value() != "" {
		count = fmt.Sprintf("KÜTÜPHANE  %d/%d parça", len(m.filtered), len(m.tracks))
	}
	lines := []string{accentStyle.Render(truncate(count, width))}
	available--

	if available > 0 && (m.searching || m.search.Value() != "") {
		if m.searching {
			if width < 3 {
				lines = append(lines, accentStyle.Render(truncate("/", width)))
			} else {
				m.search.Width = max(1, width-lipgloss.Width(m.search.Prompt))
				lines = append(lines, m.search.View())
			}
		} else {
			query := fmt.Sprintf("/ %s  ·  Esc temizle", m.search.Value())
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
		if visibleIndex == m.cursor {
			cursor = "> "
		}
		playing := "  "
		if trackIndex == m.current {
			playing = "▶ "
		}
		nameWidth := max(1, width-10)
		line := fmt.Sprintf("%s%s%3d  %s", cursor, playing, trackIndex+1, truncate(sanitize(m.tracks[trackIndex].Name), nameWidth))
		line = truncate(line, width)
		switch {
		case trackIndex == m.current:
			line = activeStyle.Render(line)
		case visibleIndex == m.cursor:
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
	lines := []string{accentStyle.Render(truncate(header, width))}
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
		if position == m.queueCursor {
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
		case position == m.queueCursor:
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
	lines := []string{accentStyle.Render(truncate(fmt.Sprintf("PLAYLISTLER  %d", len(m.playlists)), width))}
	available--
	if available <= 0 {
		return lines
	}
	if len(m.playlists) == 0 {
		return append(lines, dimStyle.Render(truncate("Kayıtlı playlist yok · sırayı S ile kaydet", width)))
	}

	start, end := visibleRange(m.playlistCursor, len(m.playlists), available)
	for position := start; position < end; position++ {
		item := m.playlists[position]
		cursor := "  "
		if position == m.playlistCursor {
			cursor = "> "
		}
		line := fmt.Sprintf("%s%3d  %s  (%d)", cursor, position+1, sanitize(item.Name), len(item.Tracks))
		line = truncate(line, width)
		if position == m.playlistCursor {
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
	title := "PLAYLIST KAYDET"
	if m.prompt == promptDeletePlaylist {
		title = "PLAYLIST SİL"
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
		m.playlistName.Width = max(1, width-lipgloss.Width(m.playlistName.Prompt))
		lines = append(lines, m.playlistName.View())
	}
	available--
	if available > 0 {
		lines = append(lines, dimStyle.Render(truncate("Enter kaydet · Esc iptal", width)))
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
		state += " · LOOP"
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
	if m.prompt == promptDeletePlaylist {
		return "Enter playlisti sil  Esc iptal"
	}
	if m.prompt == promptOverwrite {
		return "Enter üzerine yaz  Esc iptal"
	}
	if m.prompt == promptPlaylistName {
		return "Playlist adı  Enter kaydet  Esc iptal"
	}
	if m.searching {
		return "Arama yaz  Enter/Esc bitir  Esc tekrar temizle"
	}
	if width < 24 {
		return "Space n/p l Tab P q"
	}
	if width < 52 {
		return "Space n/p  l loop  Tab panel  P playlist  S kaydet  q"
	}
	switch m.panel {
	case panelQueue:
		return "Space n/p  l loop  Enter oynat  J/K taşı  x sil  c temizle  Tab kütüphane  P playlist  S kaydet  q"
	case panelPlaylists:
		return "Enter sıraya yükle  x sil  P/Esc kapat  Space n/p  l loop  q çık"
	default:
		return "Enter sırayı çal  a ekle  A tümünü ekle  / ara  Tab sıra  P playlist  S kaydet  Space n/p  l loop  q"
	}
}

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

func indexOf(values []int, wanted int) int {
	for index, value := range values {
		if value == wanted {
			return index
		}
	}
	return -1
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

func sanitize(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
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
