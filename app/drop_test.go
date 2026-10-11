// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"testing"

	"gioui.org/f32"
)

func TestDropEventQueue(t *testing.T) {
	w := new(Window)
	w.lastFrame.off.Y = 10
	for _, e := range []DropEvent{
		{Kind: DropEnter, Position: f32.Pt(1, 11)},
		{Kind: DropMove, Position: f32.Pt(2, 12)},
		{Kind: DropMove, Position: f32.Pt(3, 13)},
		{Kind: DropLeave},
		{Kind: DropEnter, Position: f32.Pt(4, 14)},
		{Kind: DropDrop, Position: f32.Pt(5, 15), Type: "text/uri-list", Data: []byte("file:///a\r\n")},
	} {
		w.processEvent(e)
	}
	want := []struct {
		kind DropKind
		pos  f32.Point
	}{
		{DropEnter, f32.Pt(1, 1)},
		{DropMove, f32.Pt(3, 3)},
		{DropLeave, f32.Point{}},
		{DropEnter, f32.Pt(4, 4)},
		{DropDrop, f32.Pt(5, 5)},
	}
	for i, exp := range want {
		evt, ok := w.nextEvent()
		if !ok {
			t.Fatalf("event %d: queue drained early", i)
		}
		e, ok := evt.(DropEvent)
		if !ok {
			t.Fatalf("event %d: got %T, want DropEvent", i, evt)
		}
		if e.Kind != exp.kind || e.Position != exp.pos {
			t.Errorf("event %d: got %v at %v, want %v at %v", i, e.Kind, e.Position, exp.kind, exp.pos)
		}
	}
	if evt, ok := w.nextEvent(); ok {
		t.Errorf("unexpected trailing event %T", evt)
	}
}
