// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"image"
	"os"

	"gioui.org/gpu/internal/driver"
	"gioui.org/internal/f32"
	"gioui.org/internal/f32color"
)

// damageTracker finds what a frame changed against the one before it, so
// that only that part of the window is drawn again. It sees a frame as a
// multiset of draw operations: an operation that was not in the last frame,
// or one of the last frame that is gone, dirties its rectangle. The region
// drawn is that, plus what the frames the back buffer is older than
// changed, which the window learns from EGL_EXT_buffer_age.
type damageTracker struct {
	prev, cur map[opSig]int
	// history holds the dirty rectangles of the latest frames, newest first.
	history  [8][]image.Rectangle
	age      int
	viewport image.Point
	// region is what the frame being drawn covers; full says it is the
	// whole viewport, and no culling applies.
	region []image.Rectangle
	full   bool
	frames int
	// clearColor is the last frame's, when its first op was a fill of the
	// whole viewport that the renderer turned into the clear: a frame drawn in
	// part paints it under its rectangles, and one of another color is whole.
	clearColor f32color.RGBA
	cleared    bool
}

// opSig is what identifies a draw operation across frames.
type opSig struct {
	clipType clipType
	clip     image.Rectangle
	layerOps int
	mat      materialType
	opaque   bool
	color    f32color.RGBA
	color1   f32color.RGBA
	color2   f32color.RGBA
	opacity  float32
	handle   any
	tex      driver.Texture
	uvTrans  f32.Affine2D
	paths    [4]pathSig
	depth    int
}

// pathSig identifies one clip of the stack by what it clips to, never by
// where its path was recorded: a path written into the frame's op.Ops is
// a new recording every frame.
type pathSig struct {
	hash           uint64
	outline        bool
	strokeWidth    float32
	sx, hx, sy, hy float32
	off            f32.Point
	bounds         f32.Rectangle
}

func sigOf(img *imageOp) opSig {
	m := img.material
	s := opSig{
		clipType: img.clipType, clip: img.clip, layerOps: img.layerOps,
		mat: m.material, opaque: m.opaque, color: m.color, color1: m.color1, color2: m.color2,
		opacity: m.opacity, handle: m.data.handle, tex: m.tex, uvTrans: m.uvTrans,
	}
	for p := img.path; p != nil; p = p.parent {
		if s.depth < len(s.paths) {
			k := p.pathKey
			s.paths[s.depth] = pathSig{hash: p.hash, outline: k.outline, strokeWidth: k.strokeWidth, sx: k.sx, hx: k.hx, sy: k.sy, hy: k.hy, off: p.off, bounds: p.bounds}
		}
		s.depth++
	}
	return s
}

// setAge tells the tracker how many frames old the back buffer the next
// frame draws into is: 1 is the frame just shown, 0 is unknown, and
// unknown means the whole viewport is drawn.
func (t *damageTracker) setAge(age int) { t.age = age }

// frame reads the frame's operations and settles the region to draw.
func (t *damageTracker) frame(ops []imageOp, layers []opacityLayer, viewport image.Point, cleared bool, clearColor f32color.RGBA) {
	if t.cur == nil {
		t.cur = map[opSig]int{}
	}
	clear(t.cur)
	// A layer's opacity lives on the layer, not on the operations it
	// composites, so each layer counts as an operation of its own.
	all := append(ops[:len(ops):len(ops)], make([]imageOp, 0, len(layers))...)
	for _, l := range layers {
		all = append(all, imageOp{clip: l.clip, layerOps: -1 - l.depth, material: material{opacity: l.opacity}})
	}
	for i := range all {
		t.cur[sigOf(&all[i])]++
	}
	var dirty []image.Rectangle
	for i := range all {
		s := sigOf(&all[i])
		if n := t.prev[s]; n > 0 {
			t.prev[s] = n - 1
			continue
		}
		dirty = addRect(dirty, all[i].clip)
	}
	for s, n := range t.prev {
		if n > 0 {
			dirty = addRect(dirty, s.clip)
		}
	}
	t.prev, t.cur = t.cur, t.prev
	whole := image.Rectangle{Max: viewport}
	for i := range dirty {
		dirty[i] = dirty[i].Intersect(whole)
	}
	first := t.prev == nil || len(t.prev) == 0 && t.frames == 0
	t.full = damageOff || first || t.age <= 0 || t.age > len(t.history)+1 || viewport != t.viewport || cleared != t.cleared || clearColor != t.clearColor
	t.cleared, t.clearColor = cleared, clearColor
	t.viewport = viewport
	t.region = append(t.region[:0], dirty...)
	for i := 0; !t.full && i < t.age-1; i++ {
		for _, r := range t.history[i] {
			t.region = addRect(t.region, r)
		}
	}
	copy(t.history[1:], t.history[:])
	t.history[0] = dirty
	if t.full {
		t.region = append(t.region[:0], whole)
	}
	t.frames++
	t.age = 0
}

// damageOff draws every frame whole, the way Gio does upstream: GIO_DAMAGE=off
// in the environment, for a driver that mishandles a frame drawn in part.
var damageOff = os.Getenv("GIO_DAMAGE") == "off"

// damageRects bounds how many rectangles a frame is drawn in: past it,
// the rectangles fold into their bounding box.
const damageRects = 16

// addRect adds r to rects, merged into the first one it overlaps.
func addRect(rects []image.Rectangle, r image.Rectangle) []image.Rectangle {
	if r.Empty() {
		return rects
	}
	for i, o := range rects {
		if o.Overlaps(r) {
			rects[i] = o.Union(r)
			return rects
		}
	}
	if len(rects) >= damageRects {
		all := r
		for _, o := range rects {
			all = all.Union(o)
		}
		return append(rects[:0], all)
	}
	return append(rects, r)
}

// overlapsAny reports whether r meets any of rects.
func overlapsAny(rects []image.Rectangle, r image.Rectangle) bool {
	for _, o := range rects {
		if o.Overlaps(r) {
			return true
		}
	}
	return false
}
