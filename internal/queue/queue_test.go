package queue

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestReplaceCopiesInputAndNavigationBounds(t *testing.T) {
	items := []int{3, 1, 4}
	q := New()
	q.Replace(items, 1)
	items[1] = 99

	if got := q.Items(); !reflect.DeepEqual(got, []int{3, 1, 4}) {
		t.Fatalf("Items() = %v", got)
	}
	if next, ok := q.NextPosition(); !ok || next != 2 {
		t.Fatalf("NextPosition() = %d, %v", next, ok)
	}
	if previous, ok := q.PreviousPosition(); !ok || previous != 0 {
		t.Fatalf("PreviousPosition() = %d, %v", previous, ok)
	}
}

func TestMovePreservesActiveTrack(t *testing.T) {
	q := New()
	q.Replace([]int{10, 20, 30, 40}, 1)
	if !q.Move(0, 3) {
		t.Fatal("Move() = false")
	}
	if got := q.Items(); !reflect.DeepEqual(got, []int{20, 30, 40, 10}) {
		t.Fatalf("Items() = %v", got)
	}
	if q.Position() != 0 {
		t.Fatalf("Position() = %d, want 0", q.Position())
	}

	if !q.Move(0, 2) {
		t.Fatal("Move(active) = false")
	}
	if q.Position() != 2 {
		t.Fatalf("Position() after moving active = %d, want 2", q.Position())
	}
}

func TestRemovingActiveDetachesPlaybackAndPreservesNavigation(t *testing.T) {
	q := New()
	q.Replace([]int{10, 20, 30}, 1)
	removed, ok := q.Remove(1)
	if !ok || removed != 20 {
		t.Fatalf("Remove() = %d, %v", removed, ok)
	}
	if q.Position() != -1 {
		t.Fatalf("Position() = %d, want detached playback", q.Position())
	}
	if next, ok := q.NextPosition(); !ok || next != 1 {
		t.Fatalf("NextPosition() = %d, %v; want position 1", next, ok)
	}
	if previous, ok := q.PreviousPosition(); !ok || previous != 0 {
		t.Fatalf("PreviousPosition() = %d, %v; want position 0", previous, ok)
	}
	if track, _ := q.ItemAt(1); track != 30 {
		t.Fatalf("next track = %d, want 30", track)
	}
}

func TestShuffleUpcomingPreservesHistoryAndActiveTrack(t *testing.T) {
	q := New()
	q.Replace([]int{10, 20, 30, 40, 50}, 1)
	if !q.ShuffleUpcoming(rand.New(rand.NewSource(4))) {
		t.Fatal("ShuffleUpcoming() = false")
	}
	items := q.Items()
	if !reflect.DeepEqual(items[:2], []int{10, 20}) {
		t.Fatalf("played prefix changed: %v", items)
	}
	if q.Position() != 1 {
		t.Fatalf("Position() = %d, want 1", q.Position())
	}
	gotUpcoming := append([]int(nil), items[2:]...)
	seen := map[int]bool{}
	for _, item := range gotUpcoming {
		seen[item] = true
	}
	for _, wanted := range []int{30, 40, 50} {
		if !seen[wanted] {
			t.Fatalf("shuffled items = %v, missing %d", items, wanted)
		}
	}
}

func TestAppendAndClear(t *testing.T) {
	q := New()
	q.Append(1)
	q.AppendMany([]int{2, 3})
	if got := q.Items(); !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("Items() = %v", got)
	}
	q.Clear()
	if q.Len() != 0 || q.Position() != -1 {
		t.Fatalf("queue after Clear() = %v at %d", q.Items(), q.Position())
	}
}
