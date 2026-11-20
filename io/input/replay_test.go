// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// A frame may replay the operations a subtree recorded earlier instead of
// running its widgets, which then ask for no events that frame, nor the
// frames after while the subtree holds still. Their handlers are still in
// the frame, so input reaching them is still theirs.

func TestPointerReachesAHandlerThroughAReplayedFrame(t *testing.T) {
	h := new(int)
	var ops op.Ops
	area := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	event.Op(&ops, h)
	area.Pop()
	f := pointer.Filter{Target: h, Kinds: pointer.Press | pointer.Release | pointer.Enter | pointer.Leave}

	var r Router
	events(&r, -1, f)
	r.Frame(&ops)
	events(&r, -1, f)
	// The subtree's operations again, twice, its widget not run.
	r.Frame(&ops)
	r.Frame(&ops)
	r.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(10, 10)},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(10, 10)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(10, 10)},
	)
	assertEventPointerTypeSequence(t, events(&r, -1, f), pointer.Enter, pointer.Press, pointer.Release)
}

func TestKeyReachesAFocusedHandlerThroughAReplayedFrame(t *testing.T) {
	h := new(int)
	var ops op.Ops
	event.Op(&ops, h)
	f := []event.Filter{key.FocusFilter{Target: h}, key.Filter{Focus: h, Name: "A"}}

	r := new(Router)
	events(r, -1, f...)
	r.Frame(&ops)
	r.Source().Execute(key.FocusCmd{Tag: h})
	events(r, -1, f...)
	r.Frame(&ops)
	events(r, -1, f...)
	// The subtree's operations again, twice, its widget not run.
	r.Frame(&ops)
	r.Frame(&ops)
	ke := key.Event{Name: "A"}
	r.Queue(ke)
	assertEventSequence(t, events(r, -1, f...), ke)
}

func TestAHandlerBackInTheFrameRoutesNothingItDidNotAskFor(t *testing.T) {
	h := new(int)
	var ops op.Ops
	area := clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops)
	event.Op(&ops, h)
	area.Pop()
	f := pointer.Filter{Target: h, Kinds: pointer.Press | pointer.Release}

	var r Router
	events(&r, -1, f)
	r.Frame(&ops)
	events(&r, -1, f)
	// Gone from the frames, the handler's state goes with it (asking for
	// events keeps it a frame more); back, it asks for nothing until its
	// widget runs again.
	r.Frame(new(op.Ops))
	r.Frame(new(op.Ops))
	r.Frame(&ops)
	r.Frame(&ops)
	r.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(10, 10)})
	for _, k := range pointerTypes(events(&r, -1, f)) {
		if k == pointer.Press {
			t.Fatal("a press reached a handler on the filter it had before it left the frame")
		}
	}
}
func TestInputQueuedSinceTheLastFrame(t *testing.T) {
	var r Router
	if r.Source().Queued() {
		t.Fatal("a new router reports queued input")
	}
	r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(1, 1)})
	if !r.Source().Queued() {
		t.Fatal("a queued pointer event is not reported")
	}
	r.Frame(new(op.Ops))
	if r.Source().Queued() {
		t.Fatal("input stays reported after the frame that saw it")
	}
	r.Queue(key.Event{Name: "A"})
	if !r.Source().Queued() {
		t.Fatal("a queued key event is not reported")
	}
}

func TestInputLeftByAFrameCountsAsQueued(t *testing.T) {
	r := new(Router)
	h := new(int)
	f := []event.Filter{key.FocusFilter{Target: h}, key.Filter{Name: "A"}}
	// Exhausting h's events defers the key to the next frame.
	events(r, -1, f...)
	r.Source().Execute(key.FocusCmd{Tag: h})
	r.Queue(key.Event{Name: "A"})
	events(r, -1, f...)
	r.Frame(new(op.Ops))
	if !r.Source().Queued() {
		t.Fatal("events a frame deferred to the next are not reported")
	}
}
