// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"image"
	"testing"

	"gioui.org/internal/f32color"
)

func colorOp(r image.Rectangle, c f32color.RGBA) imageOp {
	return imageOp{clip: r, material: material{material: materialColor, color: c, opaque: true, opacity: 1}}
}

var (
	red  = f32color.RGBA{R: 1, A: 1}
	blue = f32color.RGBA{B: 1, A: 1}
	view = image.Pt(100, 100)
)

func frameAt(t *damageTracker, ops []imageOp, view image.Point, clear f32color.RGBA) {
	t.frame(ops, nil, view, true, clear)
}

func TestFirstFrameIsWhole(t *testing.T) {
	tr := &damageTracker{}
	frameAt(tr, []imageOp{colorOp(image.Rect(0, 0, 10, 10), red)}, view, blue)
	if !tr.full {
		t.Fatal("a first frame, with nothing before it, must be drawn whole")
	}
}

func TestUnchangedOpsDirtyNothing(t *testing.T) {
	tr := &damageTracker{}
	ops := []imageOp{colorOp(image.Rect(0, 0, 10, 10), red), colorOp(image.Rect(50, 50, 60, 60), blue)}
	frameAt(tr, ops, view, blue)
	frameAt(tr, ops, view, blue)
	if tr.full || len(tr.region) != 0 {
		t.Fatalf("the same ops again dirtied %v (full %v), want nothing", tr.region, tr.full)
	}
}

func TestNewAndGoneOpsDirtyTheirRectangles(t *testing.T) {
	tr := &damageTracker{}
	frameAt(tr, []imageOp{colorOp(image.Rect(0, 0, 10, 10), red), colorOp(image.Rect(50, 50, 60, 60), blue)}, view, blue)
	frameAt(tr, []imageOp{colorOp(image.Rect(0, 0, 10, 10), red), colorOp(image.Rect(80, 80, 90, 90), blue)}, view, blue)
	want := []image.Rectangle{image.Rect(80, 80, 90, 90), image.Rect(50, 50, 60, 60)}
	if tr.full || len(tr.region) != 2 || tr.region[0] != want[0] || tr.region[1] != want[1] {
		t.Fatalf("region %v (full %v), want the op that came and the op that went: %v", tr.region, tr.full, want)
	}
}

func TestAChangedViewportOrClearIsWhole(t *testing.T) {
	tr := &damageTracker{}
	ops := []imageOp{colorOp(image.Rect(0, 0, 10, 10), red)}
	frameAt(tr, ops, view, blue)
	frameAt(tr, ops, image.Pt(200, 100), blue)
	if !tr.full {
		t.Fatal("a resized viewport must be drawn whole")
	}
	frameAt(tr, ops, image.Pt(200, 100), blue)
	frameAt(tr, ops, image.Pt(200, 100), red)
	if !tr.full {
		t.Fatal("a frame whose clear color changed must be drawn whole")
	}
}

func TestManyRectanglesFoldIntoTheirBox(t *testing.T) {
	var rects []image.Rectangle
	for i := range damageRects + 1 {
		rects = addRect(rects, image.Rect(i*10, 0, i*10+5, 5))
	}
	if len(rects) != 1 || rects[0] != image.Rect(0, 0, damageRects*10+5, 5) {
		t.Fatalf("past %d rectangles got %v, want their one bounding box", damageRects, rects)
	}
	if got := addRect(nil, image.Rectangle{}); len(got) != 0 {
		t.Fatalf("an empty rectangle was added: %v", got)
	}
}

func TestAFadingLayerDirtiesItsRectangle(t *testing.T) {
	tr := &damageTracker{}
	ops := []imageOp{colorOp(image.Rect(0, 0, 10, 10), red)}
	layer := image.Rect(20, 20, 60, 60)
	tr.frame(ops, []opacityLayer{{opacity: 1, clip: layer}}, view, true, blue)
	tr.frame(ops, []opacityLayer{{opacity: 0.5, clip: layer}}, view, true, blue)
	if tr.full || len(tr.region) != 1 || tr.region[0] != layer {
		t.Fatalf("a layer fading dirtied %v (full %v), want its rectangle %v", tr.region, tr.full, layer)
	}
}

func TestAFrameOutsideTheRetainedFramebufferIsWholeAndUntracked(t *testing.T) {
	tr := &damageTracker{}
	ops := []imageOp{colorOp(image.Rect(0, 0, 10, 10), red)}
	tr.frame(ops, nil, view, false, blue)
	if !tr.full || tr.held || tr.cur != nil || len(tr.region) != 1 || tr.region[0] != (image.Rectangle{Max: view}) {
		t.Fatalf("full %v, held %v, cur %v, region %v: want a whole, untracked frame", tr.full, tr.held, tr.cur, tr.region)
	}
}

func TestReturningToTheRetainedFramebufferIsWhole(t *testing.T) {
	tr := &damageTracker{}
	ops := []imageOp{colorOp(image.Rect(0, 0, 10, 10), red)}
	frameAt(tr, ops, view, blue)
	tr.frame(ops, nil, view, false, blue)
	frameAt(tr, ops, view, blue)
	if !tr.full {
		t.Fatal("the retained framebuffer missed a frame, so the next one must be whole")
	}
}

func TestATranslucentClearIsWhole(t *testing.T) {
	tr := &damageTracker{}
	clearColor := f32color.RGBA{R: 0.1, A: 0.3}
	ops := []imageOp{colorOp(image.Rect(0, 0, 10, 10), red)}
	frameAt(tr, ops, view, clearColor)
	frameAt(tr, ops, view, clearColor)
	if !tr.full {
		t.Fatal("a translucent ground would blend over the kept frame, so its frame must be whole")
	}
}
