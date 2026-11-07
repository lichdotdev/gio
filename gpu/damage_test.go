// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"image"
	"os"
	"testing"

	"gioui.org/internal/f32color"
)

// TestMain draws in part, as GIO_DAMAGE=on does: the tracker's diff is what
// these tests are about.
func TestMain(m *testing.M) {
	damageOn = true
	os.Exit(m.Run())
}

func colorOp(r image.Rectangle, c f32color.RGBA) imageOp {
	return imageOp{clip: r, material: material{material: materialColor, color: c, opaque: true, opacity: 1}}
}

var (
	red  = f32color.RGBA{R: 1, A: 1}
	blue = f32color.RGBA{B: 1, A: 1}
	view = image.Pt(100, 100)
)

// freshTracker draws into a buffer one frame old, as a double-buffered
// surface reports it.
func freshTracker() *damageTracker { return &damageTracker{} }

func frameAt(t *damageTracker, age int, ops []imageOp, view image.Point, clear f32color.RGBA) {
	t.setAge(age)
	t.frame(ops, nil, view, true, clear)
}

func TestFirstFrameIsWhole(t *testing.T) {
	tr := freshTracker()
	frameAt(tr, 1, []imageOp{colorOp(image.Rect(0, 0, 10, 10), red)}, view, blue)
	if !tr.full {
		t.Fatal("a first frame, with nothing before it, must be drawn whole")
	}
}

func TestUnchangedOpsDirtyNothing(t *testing.T) {
	tr := freshTracker()
	ops := []imageOp{colorOp(image.Rect(0, 0, 10, 10), red), colorOp(image.Rect(50, 50, 60, 60), blue)}
	frameAt(tr, 1, ops, view, blue)
	frameAt(tr, 1, ops, view, blue)
	if tr.full || len(tr.region) != 0 {
		t.Fatalf("the same ops again dirtied %v (full %v), want nothing", tr.region, tr.full)
	}
}

func TestNewAndGoneOpsDirtyTheirRectangles(t *testing.T) {
	tr := freshTracker()
	frameAt(tr, 1, []imageOp{colorOp(image.Rect(0, 0, 10, 10), red), colorOp(image.Rect(50, 50, 60, 60), blue)}, view, blue)
	frameAt(tr, 1, []imageOp{colorOp(image.Rect(0, 0, 10, 10), red), colorOp(image.Rect(80, 80, 90, 90), blue)}, view, blue)
	want := []image.Rectangle{image.Rect(80, 80, 90, 90), image.Rect(50, 50, 60, 60)}
	if tr.full || len(tr.region) != 2 || tr.region[0] != want[0] || tr.region[1] != want[1] {
		t.Fatalf("region %v (full %v), want the op that came and the op that went: %v", tr.region, tr.full, want)
	}
}

func TestOlderBuffersGatherTheFramesBetween(t *testing.T) {
	tr := &damageTracker{}
	a, b, c := image.Rect(0, 0, 10, 10), image.Rect(20, 20, 30, 30), image.Rect(40, 40, 50, 50)
	frameAt(tr, 1, []imageOp{colorOp(a, red)}, view, blue)
	frameAt(tr, 1, []imageOp{colorOp(b, red)}, view, blue)
	frameAt(tr, 3, []imageOp{colorOp(c, red)}, view, blue)
	if tr.full || len(tr.region) != 3 {
		t.Fatalf("drawing into a buffer three frames old covered %v (full %v), want this frame's change and the two before it", tr.region, tr.full)
	}
	frameAt(tr, 0, []imageOp{colorOp(c, red)}, view, blue)
	if !tr.full {
		t.Fatal("a buffer of unknown age must be drawn whole")
	}
}

func TestAChangedViewportOrClearIsWhole(t *testing.T) {
	tr := freshTracker()
	ops := []imageOp{colorOp(image.Rect(0, 0, 10, 10), red)}
	frameAt(tr, 1, ops, view, blue)
	frameAt(tr, 1, ops, image.Pt(200, 100), blue)
	if !tr.full {
		t.Fatal("a resized viewport must be drawn whole")
	}
	frameAt(tr, 1, ops, image.Pt(200, 100), blue)
	frameAt(tr, 1, ops, image.Pt(200, 100), red)
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
	tr := freshTracker()
	ops := []imageOp{colorOp(image.Rect(0, 0, 10, 10), red)}
	layer := image.Rect(20, 20, 60, 60)
	tr.setAge(1)
	tr.frame(ops, []opacityLayer{{opacity: 1, clip: layer}}, view, true, blue)
	tr.setAge(1)
	tr.frame(ops, []opacityLayer{{opacity: 0.5, clip: layer}}, view, true, blue)
	if tr.full || len(tr.region) != 1 || tr.region[0] != layer {
		t.Fatalf("a layer fading dirtied %v (full %v), want its rectangle %v", tr.region, tr.full, layer)
	}
}

func TestWithoutGIODamageEveryFrameIsWhole(t *testing.T) {
	damageOn = false
	t.Cleanup(func() { damageOn = true })
	tr := freshTracker()
	ops := []imageOp{colorOp(image.Rect(0, 0, 10, 10), red)}
	frameAt(tr, 1, ops, view, blue)
	frameAt(tr, 1, ops, view, blue)
	if !tr.full {
		t.Fatal("without GIO_DAMAGE=on a frame must be drawn whole, whatever the buffer age")
	}
}

func TestWithoutGIODamageTheRegionIsTheViewport(t *testing.T) {
	damageOn = false
	t.Cleanup(func() { damageOn = true })
	tr := freshTracker()
	frameAt(tr, 1, []imageOp{colorOp(image.Rect(0, 0, 10, 10), red)}, view, blue)
	if len(tr.region) != 1 || tr.region[0] != (image.Rectangle{Max: view}) {
		t.Fatalf("region %v, want the whole viewport %v", tr.region, view)
	}
}
