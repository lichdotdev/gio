// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// collectStroke records a stroked triangle, as a frame does, into a reset ops
// and collects it, returning the number of paths in the cache afterwards.
func collectStroke(d *drawOps, o *op.Ops, at f32.Point, contours bool) int {
	o.Reset()
	var p clip.Path
	p.Begin(o)
	p.MoveTo(at)
	p.LineTo(at.Add(f32.Pt(20, 0)))
	p.LineTo(at)
	if contours {
		// Away and back from a closed contour: no command, but the last
		// line starts a contour of its own.
		p.MoveTo(at.Add(f32.Pt(50, 50)))
		p.MoveTo(at)
	}
	p.LineTo(at.Add(f32.Pt(20, 20)))
	stack := clip.Stroke{Path: p.End(), Width: 2}.Op().Push(o)
	paint.PaintOp{}.Add(o)
	stack.Pop()
	view := image.Pt(100, 100)
	d.reset(view)
	d.collect(o, view)
	return len(d.pathCache.index)
}

func TestPathCacheKeepsAPathRecordedAgain(t *testing.T) {
	d := &drawOps{pathCache: newOpCache()}
	var o op.Ops
	collectStroke(d, &o, f32.Pt(10, 10), false)
	d.pathCache.frame()
	if n := collectStroke(d, &o, f32.Pt(10, 10), false); n != 1 {
		t.Fatalf("an unchanged path recorded again holds %d cache entries, want 1: it was built again", n)
	}
	d.pathCache.frame()
	if n := collectStroke(d, &o, f32.Pt(30, 10), false); n != 2 {
		t.Fatalf("a moved path holds %d cache entries, want 2: it took the old path's data", n)
	}
}

func TestPathCacheTellsContoursApart(t *testing.T) {
	d := &drawOps{pathCache: newOpCache()}
	var o op.Ops
	collectStroke(d, &o, f32.Pt(10, 10), false)
	if n := collectStroke(d, &o, f32.Pt(10, 10), true); n != 2 {
		t.Fatalf("the same segments split in two contours hold %d cache entries, want 2: they share one stroke", n)
	}
}
