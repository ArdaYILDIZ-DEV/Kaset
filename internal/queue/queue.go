package queue

// Queue stores library track indexes in playback order.
type Queue struct {
	items    []int
	position int
}

// New returns an empty queue with no active item.
func New() Queue {
	return Queue{position: -1}
}

// Len returns the number of queued tracks.
func (q Queue) Len() int {
	return len(q.items)
}

// Items returns a copy of all queued library indexes.
func (q Queue) Items() []int {
	return append([]int(nil), q.items...)
}

// Position returns the active queue position, or -1 when no queue item is active.
func (q Queue) Position() int {
	return q.position
}

// ItemAt returns the library index at position.
func (q Queue) ItemAt(position int) (int, bool) {
	if position < 0 || position >= len(q.items) {
		return 0, false
	}
	return q.items[position], true
}

// Replace replaces the queue and sets its active position. Use -1 to load a
// queue without starting playback.
func (q *Queue) Replace(items []int, position int) {
	q.items = append(q.items[:0], items...)
	q.position = clampPosition(position, len(q.items))
}

// Append adds one library track index to the end of the queue.
func (q *Queue) Append(trackIndex int) {
	q.items = append(q.items, trackIndex)
}

// AppendMany adds library track indexes to the end in their current order.
func (q *Queue) AppendMany(trackIndexes []int) {
	q.items = append(q.items, trackIndexes...)
}

// SetPosition marks a queue item as active and returns its library index.
func (q *Queue) SetPosition(position int) (int, bool) {
	trackIndex, ok := q.ItemAt(position)
	if !ok {
		return 0, false
	}
	q.position = position
	return trackIndex, true
}

// ResetPosition keeps the queue but removes its active item.
func (q *Queue) ResetPosition() {
	q.position = -1
}

// NextPosition returns the next playable queue position without modifying state.
func (q Queue) NextPosition() (int, bool) {
	next := q.position + 1
	if next < 0 {
		next = 0
	}
	return next, next < len(q.items)
}

// PreviousPosition returns the previous playable queue position without modifying state.
func (q Queue) PreviousPosition() (int, bool) {
	previous := q.position - 1
	return previous, previous >= 0 && previous < len(q.items)
}

// Move relocates one queue item while preserving the active track identity.
func (q *Queue) Move(from, to int) bool {
	if from < 0 || from >= len(q.items) || to < 0 || to >= len(q.items) || from == to {
		return false
	}

	item := q.items[from]
	if from < to {
		copy(q.items[from:to], q.items[from+1:to+1])
	} else {
		copy(q.items[to+1:from+1], q.items[to:from])
	}
	q.items[to] = item

	switch {
	case q.position == from:
		q.position = to
	case from < q.position && to >= q.position:
		q.position--
	case from > q.position && to <= q.position:
		q.position++
	}
	return true
}

// Remove deletes one queue item. Removing the active item keeps playback alive;
// the following item becomes the next item that NextPosition returns.
func (q *Queue) Remove(position int) (int, bool) {
	if position < 0 || position >= len(q.items) {
		return 0, false
	}

	removed := q.items[position]
	q.items = append(q.items[:position], q.items[position+1:]...)
	switch {
	case len(q.items) == 0:
		q.position = -1
	case position < q.position:
		q.position--
	case position == q.position:
		q.position = position - 1
	}
	return removed, true
}

// Clear removes every queued item.
func (q *Queue) Clear() {
	q.items = nil
	q.position = -1
}

func clampPosition(position, length int) int {
	if position < 0 || length == 0 {
		return -1
	}
	if position >= length {
		return length - 1
	}
	return position
}
