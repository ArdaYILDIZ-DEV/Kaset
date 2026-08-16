package tui

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"kaset/internal/player"
)

// playLibraryContext replaces the queue with visible results and starts at the cursor.
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
		m.setError("Çalma sırasına eklenecek parça yok")
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
		m.setError("Çalma sırasına eklenecek arama sonucu yok")
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

// playQueueAt marks a queue item active only after mpv accepts the track.
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

// playTrack clears stale metadata when loading a new library track.
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
	m.hasMetadataTitle = false
	m.position = 0
	m.duration = 0
	m.paused = false
	m.clearFeedback()
	return true
}

func (m *Model) playNext() {
	position, ok := m.queue.NextPosition()
	if !ok && m.repeatQueue && m.queue.Len() > 0 {
		position, ok = 0, true
	}
	if !ok {
		m.setNotice("Çalma sırasının sonu")
		return
	}
	m.playQueueAt(position)
}

func (m *Model) playPrevious() {
	position, ok := m.queue.PreviousPosition()
	if !ok && m.repeatQueue && m.queue.Len() > 0 {
		position, ok = m.queue.Len()-1, true
	}
	if !ok {
		m.setNotice("Çalma sırasının başı")
		return
	}
	m.playQueueAt(position)
}

func (m *Model) toggleCurrentLoop() {
	if m.loopCurrent {
		m.loopCurrent = false
		m.setNotice("Tek parça döngüsü kapatıldı")
		return
	}
	if m.current < 0 {
		m.setError("Döngü için çalan bir parça yok")
		return
	}
	m.loopCurrent = true
	m.setNotice("Tek parça döngüsü açıldı")
}

func (m *Model) toggleQueueRepeat() {
	m.repeatQueue = !m.repeatQueue
	if m.repeatQueue {
		m.setNotice("Çalma sırası döngüsü açıldı")
	} else {
		m.setNotice("Çalma sırası döngüsü kapatıldı")
	}
}

func (m *Model) shuffleUpcoming() {
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	if !m.queue.ShuffleUpcoming(random) {
		m.setNotice("Karıştırılacak en az iki sıradaki parça yok")
		return
	}
	m.loadedPlaylist = ""
	m.setNotice("Sıradaki parçalar karıştırıldı")
}

func (m *Model) handlePlayerEvent(event player.Event) {
	switch event.Type {
	case player.EventProperty:
		m.handleProperty(event.Name, event.Data)
	case player.EventEndFile:
		m.handleEndFile(event)
	case player.EventError:
		if event.Err != nil {
			m.setError(event.Err.Error())
		}
	}
}

// handleEndFile applies error skipping, track looping, and queue repeat rules.
func (m *Model) handleEndFile(event player.Event) {
	if event.Reason == "error" {
		failed := m.title
		if failed == "" {
			failed = "Parça"
		}
		detail := event.FileError
		if detail == "" {
			detail = "bilinmeyen oynatma hatası"
		}
		message := fmt.Sprintf("%s çalınamadı: %s", failed, detail)
		position, ok := m.queue.NextPosition()
		if ok {
			if m.playQueueAt(position) {
				m.setNotice(message + "; sıradaki parçaya geçildi")
				return
			}
			// A failed load still consumes this queue entry so the next manual
			// transition does not retry the same broken track.
			_, _ = m.queue.SetPosition(position)
		}
		m.current = -1
		m.paused = true
		m.setError(message)
		return
	}
	if event.Reason != "eof" {
		return
	}
	if m.loopCurrent && m.current >= 0 {
		m.playTrack(m.current)
		return
	}
	position, ok := m.queue.NextPosition()
	if !ok && m.repeatQueue && m.queue.Len() > 0 {
		position, ok = 0, true
	}
	if ok {
		m.playQueueAt(position)
		return
	}
	m.current = -1
	m.paused = true
	m.position = m.duration
}

// handleProperty mirrors observed mpv state into fields used by the view.
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
		// mpv delivers media-title and metadata events in an undefined order.
		// Keep an explicit metadata title when present; media-title (usually the
		// file name) only fills in when no track title came from metadata.
		if value, ok := data.(string); ok && value != "" && !m.hasMetadataTitle {
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
					m.hasMetadataTitle = true
				}
			case "artist":
				m.artist = text
			case "album":
				m.album = text
			}
		}
	}
}
