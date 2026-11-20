// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// A frame may replay the operations a subtree recorded earlier instead of
// running its widgets, which then ask for no events that frame, nor the
// frames after while the subtree holds still.

// area is a 100x100 handler for h, under a KeepOp when kept.
func area(h event.Tag, kept bool) *op.Ops {
	var ops op.Ops
	if kept {
		defer KeepOp{}.Push(&ops).Pop()
	}
	defer clip.Rect(image.Rect(0, 0, 100, 100)).Push(&ops).Pop()
	event.Op(&ops, h)
	return &ops
}

func click(r *Router) {
	r.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(10, 10)},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(10, 10)},
		pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(10, 10)},
	)
}

func TestPointerReachesAKeptHandlerThroughReplayedFrames(t *testing.T) {
	h := new(int)
	ops := area(h, true)
	f := pointer.Filter{Target: h, Kinds: pointer.Press | pointer.Release | pointer.Enter | pointer.Leave}

	var r Router
	events(&r, -1, f)
	r.Frame(ops)
	events(&r, -1, f)
	// The subtree's operations again, twice, its widget not run.
	r.Frame(ops)
	r.Frame(ops)
	click(&r)
	assertEventPointerTypeSequence(t, events(&r, -1, f), pointer.Enter, pointer.Press, pointer.Release)
}

func TestKeyReachesAKeptFocusedHandlerThroughReplayedFrames(t *testing.T) {
	h := new(int)
	var ops op.Ops
	keep := KeepOp{}.Push(&ops)
	event.Op(&ops, h)
	keep.Pop()
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
	if !r.Source().Focused(h) {
		t.Fatal("a kept handler lost the focus through replayed frames")
	}
	ke := key.Event{Name: "A"}
	r.Queue(ke)
	assertEventSequence(t, events(r, -1, f...), ke)
}

// A handler outside a KeepOp that stops asking, as a dialog's scrim through
// its fade-out, stops taking events: it still covers what is under it, and
// routes nothing.
func TestAHandlerThatStopsAskingOutsideAKeepOpRoutesNothing(t *testing.T) {
	h := new(int)
	ops := area(h, false)
	f := pointer.Filter{Target: h, Kinds: pointer.Press | pointer.Release}

	var r Router
	events(&r, -1, f)
	r.Frame(ops)
	events(&r, -1, f)
	r.Frame(ops)
	r.Frame(ops)
	click(&r)
	for _, k := range pointerTypes(events(&r, -1, f)) {
		if k == pointer.Press {
			t.Fatal("a handler that stopped asking took a press two frames later")
		}
	}
}

// A kept handler whose widget runs again routes by what it asks for then.
func TestAKeptHandlerThatAsksAgainRoutesByItsNewFilter(t *testing.T) {
	h := new(int)
	ops := area(h, true)
	press := pointer.Filter{Target: h, Kinds: pointer.Press}
	release := pointer.Filter{Target: h, Kinds: pointer.Release}

	var r Router
	events(&r, -1, press)
	r.Frame(ops)
	r.Frame(ops)
	events(&r, -1, release)
	r.Frame(ops)
	r.Frame(ops)
	click(&r)
	if k := pointerTypes(events(&r, -1, press)); len(k) != 0 {
		t.Fatalf("a kept handler that asked for releases since took %v", k)
	}
}

func TestInputQueuedSinceTheLastFrame(t *testing.T) {
	var r Router
	if r.Source().Queued(time.Now()) {
		t.Fatal("a new router reports queued input")
	}
	r.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(1, 1)})
	if !r.Source().Queued(time.Now()) {
		t.Fatal("a queued pointer event is not reported")
	}
	r.Frame(new(op.Ops))
	if r.Source().Queued(time.Now()) {
		t.Fatal("input stays reported after the frame that saw it")
	}
	r.Queue(key.Event{Name: "A"})
	if !r.Source().Queued(time.Now()) {
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
	if !r.Source().Queued(time.Now()) {
		t.Fatal("events a frame deferred to the next are not reported")
	}
}
