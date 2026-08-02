package queue

import "math/rand"

// Queue stores library track indexes in playback order.
type Queue struct {
	items        []int
	position     int
	detachedNext int
}

// New returns an empty queue with no active item.
func New() Queue {
	return Queue{position: -1, detachedNext: -1}
}

// Len returns the number of queued tracks.
func (q Queue) Len() int {
	return len(q.items)
}

// Items returns a copy of all queued library indexes.
func (q Queue) Items() []int {
	return append([]int(nil), q.items...)
}

// Position returns the active queue position, or -1 when playback is detached.
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
	q.detachedNext = -1
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
	q.detachedNext = -1
	return trackIndex, true
}

// ResetPosition keeps the queue but removes its active and detached positions.
func (q *Queue) ResetPosition() {
	q.position = -1
	q.detachedNext = -1
}

// NextPosition returns the next playable queue position without modifying state.
func (q Queue) NextPosition() (int, bool) {
	next := 0
	switch {
	case q.position >= 0:
		next = q.position + 1
	case q.detachedNext >= 0:
		next = q.detachedNext
	}
	return next, next >= 0 && next < len(q.items)
}

// PreviousPosition returns the previous playable queue position without modifying state.
func (q Queue) PreviousPosition() (int, bool) {
	previous := -1
	switch {
	case q.position >= 0:
		previous = q.position - 1
	case q.detachedNext >= 0:
		previous = q.detachedNext - 1
	}
	return previous, previous >= 0 && previous < len(q.items)
}

// Move relocates one queue item while preserving active playback state.
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
	if q.detachedNext >= 0 {
		q.detachedNext = movedBoundary(q.detachedNext, from, to)
	}
	return true
}

// Remove deletes one queue item. Removing the active item detaches playback
// while preserving the boundary between previous and upcoming tracks.
func (q *Queue) Remove(position int) (int, bool) {
	if position < 0 || position >= len(q.items) {
		return 0, false
	}

	removed := q.items[position]
	q.items = append(q.items[:position], q.items[position+1:]...)
	switch {
	case len(q.items) == 0:
		q.position = -1
		q.detachedNext = -1
	case position < q.position:
		q.position--
	case position == q.position:
		q.position = -1
		q.detachedNext = position
	case q.detachedNext >= 0 && position < q.detachedNext:
		q.detachedNext--
	}
	return removed, true
}

// ShuffleUpcoming randomizes tracks that have not played yet.
func (q *Queue) ShuffleUpcoming(random *rand.Rand) bool {
	if random == nil {
		return false
	}
	start := 0
	switch {
	case q.position >= 0:
		start = q.position + 1
	case q.detachedNext >= 0:
		start = q.detachedNext
	}
	if len(q.items)-start < 2 {
		return false
	}
	random.Shuffle(len(q.items)-start, func(i, j int) {
		q.items[start+i], q.items[start+j] = q.items[start+j], q.items[start+i]
	})
	return true
}

// Clear removes every queued item.
func (q *Queue) Clear() {
	q.items = nil
	q.position = -1
	q.detachedNext = -1
}

func movedBoundary(boundary, from, to int) int {
	switch {
	case from < boundary && to >= boundary:
		return boundary - 1
	case from >= boundary && to < boundary:
		return boundary + 1
	default:
		return boundary
	}
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
