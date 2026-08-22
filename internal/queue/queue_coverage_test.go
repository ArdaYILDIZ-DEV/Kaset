package queue

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestDetachedNextPositionBranches(t *testing.T) {
	q := New()
	if _, ok := q.DetachedNextPosition(); ok {
		t.Fatal("empty queue should not have detached position")
	}
	q.Replace([]int{10, 20, 30}, 1)
	if _, ok := q.DetachedNextPosition(); ok {
		t.Fatal("should not be detached while active")
	}
	q.Remove(1)
	if next, ok := q.DetachedNextPosition(); !ok || next != 1 {
		t.Fatalf("DetachedNextPosition() = %d, %v; want 1, true", next, ok)
	}
	q.SetPosition(1)
	if _, ok := q.DetachedNextPosition(); ok {
		t.Fatal("detached should be cleared after SetPosition")
	}
}

func TestSetDetachedNextPositionBranches(t *testing.T) {
	q := New()
	q.Replace([]int{10, 20, 30}, 0)
	if ok := q.SetDetachedNextPosition(1); ok {
		t.Fatal("SetDetachedNextPosition should fail while active")
	}
	q.ResetPosition()
	if ok := q.SetDetachedNextPosition(-1); ok {
		t.Fatal("negative position should be rejected")
	}
	if ok := q.SetDetachedNextPosition(5); ok {
		t.Fatal("position beyond length should be rejected")
	}
	if ok := q.SetDetachedNextPosition(2); !ok {
		t.Fatal("valid detached position was rejected")
	}
	if ok := q.SetDetachedNextPosition(3); !ok {
		t.Fatal("detached at Len should be accepted")
	}
	if next, ok := q.DetachedNextPosition(); !ok || next != 3 {
		t.Fatalf("DetachedNextPosition() = %d, %v; want 3", next, ok)
	}
}

func TestSetPositionAndResetPosition(t *testing.T) {
	q := New()
	q.Replace([]int{10, 20, 30}, -1)
	if _, ok := q.SetPosition(5); ok {
		t.Fatal("out-of-range SetPosition should fail")
	}
	if _, ok := q.SetPosition(-1); ok {
		t.Fatal("negative SetPosition should fail")
	}
	if track, ok := q.SetPosition(1); !ok || track != 20 {
		t.Fatalf("SetPosition(1) = %d, %v; want 20", track, ok)
	}
	if q.Position() != 1 {
		t.Fatalf("Position() = %d, want 1", q.Position())
	}
	if next, ok := q.DetachedNextPosition(); ok {
		t.Fatalf("should not be detached after SetPosition, got %d", next)
	}
	q.ResetPosition()
	if q.Position() != -1 {
		t.Fatalf("after ResetPosition Position = %d, want -1", q.Position())
	}
	if _, ok := q.DetachedNextPosition(); ok {
		t.Fatal("should not be detached after ResetPosition")
	}
}

func TestItemAtBranches(t *testing.T) {
	q := New()
	q.Replace([]int{10, 20}, 0)
	if _, ok := q.ItemAt(-1); ok {
		t.Fatal("negative ItemAt should fail")
	}
	if _, ok := q.ItemAt(2); ok {
		t.Fatal("out-of-range ItemAt should fail")
	}
	if track, ok := q.ItemAt(0); !ok || track != 10 {
		t.Fatalf("ItemAt(0) = %d, %v", track, ok)
	}
}

func TestAppendDetachedAfterLast(t *testing.T) {
	q := New()
	q.Replace([]int{10, 20, 30}, 2)
	q.Remove(2)
	if next, ok := q.DetachedNextPosition(); !ok || next != 2 {
		t.Fatalf("after removing last, detached = %d, %v", next, ok)
	}
	// Append while detached == len
	q.Append(40)
	if next, ok := q.DetachedNextPosition(); !ok || next != 3 {
		t.Fatalf("after Append detached = %d, %v; want 3", next, ok)
	}
	if got := q.Items(); !reflect.DeepEqual(got, []int{10, 20, 40}) {
		t.Fatalf("Items() = %v, want [10 20 40]", got)
	}
	q.ResetPosition()
	q.Append(50)
	if _, ok := q.DetachedNextPosition(); ok {
		t.Fatal("Append without detached should not create detached")
	}

	// AppendMany while detached == len
	q.Replace([]int{1, 2, 3}, 2)
	q.Remove(2)
	q.AppendMany([]int{4, 5})
	if next, ok := q.DetachedNextPosition(); !ok || next != 4 {
		t.Fatalf("after AppendMany detached = %d, %v; want 4", next, ok)
	}
	if got := q.Items(); !reflect.DeepEqual(got, []int{1, 2, 4, 5}) {
		t.Fatalf("Items() = %v", got)
	}
}

func TestRemoveBranches(t *testing.T) {
	q := New()
	if _, ok := q.Remove(-1); ok {
		t.Fatal("negative Remove should fail")
	}
	if _, ok := q.Remove(0); ok {
		t.Fatal("Remove on empty queue should fail")
	}
	q.Replace([]int{10}, 0)
	q.Remove(0)
	if q.Len() != 0 || q.Position() != -1 {
		t.Fatalf("after removing single element Len=%d Position=%d", q.Len(), q.Position())
	}
	if _, ok := q.DetachedNextPosition(); ok {
		t.Fatal("detached should be cleared after queue becomes empty")
	}

	q.Replace([]int{10, 20, 30, 40}, 2)
	// position < q.position
	removed, ok := q.Remove(1)
	if !ok || removed != 20 || q.Position() != 1 {
		t.Fatalf("Remove(position<active) Position=%d removed=%d", q.Position(), removed)
	}
	// detachedNext >=0 && position < detachedNext
	q.Remove(1) // remove active 30, detached=1
	if _, ok := q.DetachedNextPosition(); !ok {
		t.Fatal("should be detached after removing active")
	}
	// remove element before detached -> detached should decrement
	detachedBefore, _ := q.DetachedNextPosition()
	q.Remove(0)
	detachedAfter, ok := q.DetachedNextPosition()
	if !ok || detachedAfter != detachedBefore-1 {
		t.Fatalf("detached after Remove %d -> %d", detachedBefore, detachedAfter)
	}

	// position > detachedNext should not affect detached
	q2 := New()
	q2.Replace([]int{10, 20, 30, 40}, 1)
	q2.Remove(1)
	before, _ := q2.DetachedNextPosition()
	q2.Remove(2) // remove after detached
	after, _ := q2.DetachedNextPosition()
	if after != before {
		t.Fatalf("removing after detached should not change detached: %d -> %d", before, after)
	}
}

func TestMoveInvalidAndMovedPositionBranches(t *testing.T) {
	q := New()
	q.Replace([]int{10, 20, 30}, 0)
	if q.Move(-1, 1) {
		t.Fatal("Move with negative from should fail")
	}
	if q.Move(0, 5) {
		t.Fatal("Move with out-of-range to should fail")
	}
	if q.Move(1, 1) {
		t.Fatal("Move with same from and to should fail")
	}
	if q.Move(0, 3) {
		t.Fatal("Move with to == len should fail")
	}
	// from < position && to >= position
	q.Replace([]int{10, 20, 30, 40, 50}, 2) // active 30
	q.Move(0, 4)                           // move 10 to end
	if q.Position() != 1 {
		t.Fatalf("movedPosition from<pos && to>=pos expected 1, got %d", q.Position())
	}
	// from > position && to <= position
	q.Replace([]int{10, 20, 30, 40, 50}, 1) // active 20
	q.Move(4, 0)                           // move 50 to front
	if q.Position() != 2 {
		t.Fatalf("movedPosition from>pos && to<=pos expected 2, got %d", q.Position())
	}
	// position == from
	q.Replace([]int{10, 20, 30, 40}, 2)
	q.Move(2, 0)
	if q.Position() != 0 {
		t.Fatalf("active element moved, Position %d, want 0", q.Position())
	}
	// default: from and to outside active, position unchanged
	q.Replace([]int{10, 20, 30, 40, 50}, 0)
	q.Move(3, 4)
	if q.Position() != 0 {
		t.Fatalf("unrelated Move should keep Position %d, want 0", q.Position())
	}
	// detachedWasAfterLast
	q.Replace([]int{10, 20, 30}, 2)
	q.Remove(2)
	beforeLen := q.Len()
	if _, ok := q.DetachedNextPosition(); !ok {
		t.Fatalf("detached beforeLen ok=%v", ok)
	}
	// when detached == len, Move should keep detached at len
	q.Move(0, 1)
	if next, ok := q.DetachedNextPosition(); !ok || next != beforeLen {
		t.Fatalf("detachedWasAfterLast detached %d, want %d", next, beforeLen)
	}
	// when detached < len, move should update detached via movedPosition
	q2 := New()
	q2.Replace([]int{10, 20, 30, 40}, 1)
	q2.Remove(1) // detached 1
	q2.Move(2, 0) // move 40 to front, detached 1 -> 2
	if next, ok := q2.DetachedNextPosition(); !ok || next != 2 {
		t.Fatalf("detached Move result %d, %v; want 2, true", next, ok)
	}
}

func TestClampPositionBranches(t *testing.T) {
	q := New()
	q.Replace([]int{10, 20, 30}, -5)
	if q.Position() != -1 {
		t.Fatalf("negative clamp Position %d, want -1", q.Position())
	}
	q.Replace([]int{10, 20}, 10)
	if q.Position() != 1 {
		t.Fatalf("out-of-range clamp Position %d, want 1", q.Position())
	}
	q.Replace([]int{10, 20, 30}, 1)
	if q.Position() != 1 {
		t.Fatalf("valid clamp Position %d, want 1", q.Position())
	}
	q.Replace(nil, 0)
	if q.Position() != -1 {
		t.Fatalf("empty list clamp Position %d, want -1", q.Position())
	}
}

func TestShuffleUpcomingBranches(t *testing.T) {
	q := New()
	if q.ShuffleUpcoming(nil) {
		t.Fatal("Shuffle with nil rand should fail")
	}
	q.Replace([]int{10, 20}, 0)
	if q.ShuffleUpcoming(rand.New(rand.NewSource(1))) {
		t.Fatal("Shuffle with single upcoming should fail")
	}
	q.Replace([]int{10, 20, 30}, -1) // no detached, position -1 => start 0
	if !q.ShuffleUpcoming(rand.New(rand.NewSource(2))) {
		t.Fatal("ShuffleUpcoming without detached failed")
	}
	q.Replace([]int{10, 20, 30, 40}, 0)
	q.Remove(0) // detached 0
	if !q.ShuffleUpcoming(rand.New(rand.NewSource(3))) {
		t.Fatal("ShuffleUpcoming with detached failed")
	}
	// insufficient upcoming after detached
	q2 := New()
	q2.Replace([]int{10, 20}, 1)
	q2.Remove(1) // detached 1, len 1 => upcoming 0
	if q2.ShuffleUpcoming(rand.New(rand.NewSource(4))) {
		t.Fatal("Shuffle with insufficient upcoming after detached should fail")
	}
}

func TestNextAndPreviousPositionDetached(t *testing.T) {
	q := New()
	q.Replace([]int{10, 20, 30}, -1)
	if next, ok := q.NextPosition(); !ok || next != 0 {
		t.Fatalf("empty queue NextPosition %d, %v; want 0, true", next, ok)
	}
	if _, ok := q.PreviousPosition(); ok {
		t.Fatal("empty queue PreviousPosition should fail")
	}
	q.Replace([]int{10, 20, 30}, 1)
	q.Remove(1)
	if next, ok := q.NextPosition(); !ok || next != 1 {
		t.Fatalf("detached NextPosition %d, %v", next, ok)
	}
	if prev, ok := q.PreviousPosition(); !ok || prev != 0 {
		t.Fatalf("detached PreviousPosition %d, %v", prev, ok)
	}
	q2 := New()
	q2.Replace([]int{10, 20, 30}, 2)
	q2.Remove(2) // detached == len
	if _, ok := q2.NextPosition(); ok {
		t.Fatal("detached == len NextPosition should fail")
	}
	if prev, ok := q2.PreviousPosition(); !ok || prev != 1 {
		t.Fatalf("detached == len PreviousPosition %d, %v", prev, ok)
	}
}
