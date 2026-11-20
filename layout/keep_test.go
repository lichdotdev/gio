// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/op"
)

// keptSubtree records, under a KeepOp, a widget that asks for a frame at at,
// as a cache records a subtree it then replays.
func keptSubtree(r *input.Router, at time.Time) op.CallOp {
	var rec op.Ops
	gtx := Context{Ops: &rec, Source: r.Source()}
	m := op.Record(gtx.Ops)
	keep := input.KeepOp{}.Push(gtx.Ops)
	gtx.Execute(op.InvalidateCmd{At: at})
	keep.Pop()
	return m.Stop()
}

// frame draws a frame that replays call, and asks for its own frame at
// animation when it is not zero, as another animation in the window does.
func frame(r *input.Router, call op.CallOp, animation time.Time) {
	var ops op.Ops
	if !animation.IsZero() {
		r.Source().Execute(op.InvalidateCmd{At: animation})
	}
	call.Add(&ops)
	r.Frame(&ops)
}

// A tooltip's delay asks for the frame that shows it; replayed while
// another animation draws the window, the subtree still gets that frame.
func TestAReplayedSubtreeKeepsTheFrameItAskedFor(t *testing.T) {
	r := new(input.Router)
	now := time.Now()
	show := now.Add(500 * time.Millisecond)
	call := keptSubtree(r, show)
	frame(r, call, time.Time{})
	if at, ok := r.WakeupTime(); !ok || !at.Equal(show) {
		t.Fatalf("the recording frame asks for %v (%v), want %v", at, ok, show)
	}
	frame(r, call, now.Add(16*time.Millisecond))
	r.WakeupTime()
	frame(r, call, time.Time{})
	if at, ok := r.WakeupTime(); !ok || !at.Equal(show) {
		t.Fatalf("after replayed frames the window wakes at %v (%v), want the tooltip's %v", at, ok, show)
	}
}

// A blink asks for its next phase; once that time comes in a replayed
// frame, the frame reports input, so the cache lays the subtree out again.
func TestAReplayedFrameWhoseWakeupFellDueReportsInput(t *testing.T) {
	r := new(input.Router)
	due := time.Now().Add(-time.Millisecond)
	call := keptSubtree(r, due)
	frame(r, call, time.Time{})
	frame(r, call, time.Now().Add(16*time.Millisecond))
	if !r.Source().Queued() {
		t.Fatal("a replayed subtree's due frame is not reported, so it is never laid out again")
	}
}

func TestAFutureReplayedWakeupIsNotInput(t *testing.T) {
	r := new(input.Router)
	call := keptSubtree(r, time.Now().Add(time.Hour))
	frame(r, call, time.Time{})
	frame(r, call, time.Time{})
	if r.Source().Queued() {
		t.Fatal("a replayed wakeup an hour away reports input")
	}
}

// Outside a KeepOp a frame request stays a command for the frame it was
// made in, as before.
func TestAFrameRequestOutsideAKeepOpIsNotReplayed(t *testing.T) {
	r := new(input.Router)
	var rec op.Ops
	gtx := Context{Ops: &rec, Source: r.Source()}
	m := op.Record(gtx.Ops)
	gtx.Execute(op.InvalidateCmd{At: time.Now().Add(time.Second)})
	call := m.Stop()
	frame(r, call, time.Time{})
	r.WakeupTime()
	frame(r, call, time.Time{})
	if at, ok := r.WakeupTime(); ok {
		t.Fatalf("a request made outside a KeepOp was replayed: wakes at %v", at)
	}
}
