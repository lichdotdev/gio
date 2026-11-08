// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"image"

	"gioui.org/gpu/internal/driver"
	"gioui.org/internal/f32"
	"gioui.org/internal/f32color"
)

// damageTracker finds what a frame changed against the one before it, so
// that only that part of the retained framebuffer is drawn again. It sees a
// frame as a multiset of draw operations: an operation that was not in the
// last frame, or one of the last frame that is gone, dirties its rectangle.
type damageTracker struct {
	prev, cur map[opSig]int
	viewport  image.Point
	// region is what the frame being drawn covers; full says it is the
	// whole viewport, and no culling applies.
	region []image.Rectangle
	full   bool
	// held says the frame just tracked was drawn into the retained
	// framebuffer, which then holds it for the next one.
	held       bool
	clearColor f32color.RGBA
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

// frame reads the frame's operations and settles the region to draw. A
// frame that does not go into the retained framebuffer is whole, and hashing
// its operations would find what nothing reads.
func (t *damageTracker) frame(ops []imageOp, layers []opacityLayer, viewport image.Point, retained bool, clearColor f32color.RGBA) {
	whole := image.Rectangle{Max: viewport}
	if !retained {
		t.held, t.full = false, true
		t.region = append(t.region[:0], whole)
		return
	}
	if t.cur == nil {
		t.cur = map[opSig]int{}
	}
	clear(t.cur)
	dirty := t.region[:0]
	mark := func(img *imageOp) {
		s := sigOf(img)
		t.cur[s]++
		if n := t.prev[s]; n > 0 {
			t.prev[s] = n - 1
			return
		}
		dirty = addRect(dirty, img.clip.Intersect(whole))
	}
	for i := range ops {
		mark(&ops[i])
	}
	// A layer's opacity lives on the layer, not on the operations it
	// composites, so each layer counts as an operation of its own.
	for _, l := range layers {
		mark(&imageOp{clip: l.clip, layerOps: -1 - l.depth, material: material{opacity: l.opacity}})
	}
	for s, n := range t.prev {
		if n > 0 {
			dirty = addRect(dirty, s.clip.Intersect(whole))
		}
	}
	t.prev, t.cur = t.cur, t.prev
	// A translucent ground blends over what the framebuffer kept instead
	// of replacing it, so only a whole frame gets a translucent clear right.
	t.full = !t.held || viewport != t.viewport || clearColor != t.clearColor || clearColor.A != 1
	t.held, t.viewport, t.clearColor = true, viewport, clearColor
	if t.full {
		dirty = append(dirty[:0], whole)
	}
	t.region = dirty
}

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
