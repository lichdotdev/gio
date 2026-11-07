// SPDX-License-Identifier: Unlicense OR MIT

package rendertest

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

// shapeTolerance is how far, out of 255, a pixel's coverage where the
// renderer draws a shape as a quad may stray from the stencilled path's.
// The quad computes the area exactly for straight edges and nearly so
// around curves; the stencil strays as far near steep edges.
const shapeTolerance = 26

// overlapPixels is how many pixels around one place where a stroked path
// overlaps itself (a sharp corner, the two ends of a ring) may differ
// further: the stencil counts the overlap twice there, the quad once.
const overlapPixels = 6

const shapeWindow = 256

// shapeCase draws one shape as a clip.RRect or clip.Arc, which the
// renderer may draw as a quad, and as an untagged path, which it always
// stencils.
type shapeCase struct {
	name     string
	quad     func(o *op.Ops) clip.Op
	path     func(o *op.Ops) clip.Op
	layer    bool
	overlaps int
}

func TestShapesMatchPaths(t *testing.T) {
	for _, s := range []float32{1, 2} {
		for _, c := range shapeCases() {
			t.Run(fmt.Sprintf("%s@%v", c.name, s), func(t *testing.T) {
				quad := drawShape(t, s, c.layer, c.quad)
				path := drawShape(t, s, c.layer, c.path)
				worst, at, over := 0, image.Point{}, 0
				for y := range shapeWindow {
					for x := range shapeWindow {
						d := coverageDiff(quad.RGBAAt(x, y), path.RGBAAt(x, y))
						if d > worst {
							worst, at = d, image.Pt(x, y)
						}
						if d > shapeTolerance {
							over++
						}
					}
				}
				if over > c.overlaps*overlapPixels {
					t.Errorf("%d pixels stray past %d, the worst %d at %v: %v, want %v", over, shapeTolerance, worst, at, quad.RGBAAt(at.X, at.Y), path.RGBAAt(at.X, at.Y))
				}
			})
		}
	}
}

func shapeCases() []shapeCase {
	var cases []shapeCase
	rrects := []clip.RRect{
		clip.UniformRRect(image.Rect(4, 4, 44, 28), 4),
		clip.UniformRRect(image.Rect(10, 10, 90, 50), 8),
		clip.UniformRRect(image.Rect(20, 20, 60, 60), 20),
		clip.UniformRRect(image.Rect(5, 70, 65, 90), 10),
		{Rect: image.Rect(8, 8, 70, 50), SE: 0, NE: 6, SW: 12, NW: 3},
		clip.UniformRRect(image.Rect(3, 3, 9, 9), 3),
		clip.UniformRRect(image.Rect(30, 30, 100, 110), 0),
	}
	for _, rr := range rrects {
		cases = append(cases, shapeCase{
			name: fmt.Sprintf("fill%v", rr),
			quad: func(o *op.Ops) clip.Op { return clip.Outline{Path: rr.Path(o)}.Op() },
			path: func(o *op.Ops) clip.Op { return clip.Outline{Path: rrectPath(o, rr)}.Op() },
		})
		for _, w := range []float32{1, 1.5, 2} {
			cases = append(cases, shapeCase{
				name:     fmt.Sprintf("border%v/%v", rr, w),
				quad:     func(o *op.Ops) clip.Op { return clip.Stroke{Path: rr.Path(o), Width: w}.Op() },
				path:     func(o *op.Ops) clip.Op { return clip.Stroke{Path: rrectPath(o, rr), Width: w}.Op() },
				overlaps: sharpCorners(rr),
			})
		}
	}
	arcs := []clip.Arc{
		{Center: f32.Pt(40, 40), Radius: 10, Start: 0, Sweep: math.Pi / 2},
		{Center: f32.Pt(40, 40), Radius: 10, Start: 2.2, Sweep: math.Pi / 2},
		{Center: f32.Pt(40, 40), Radius: 10, Start: 0, Sweep: 2 * math.Pi},
		{Center: f32.Pt(60, 50), Radius: 30, Start: math.Pi, Sweep: 4},
		{Center: f32.Pt(30, 30), Radius: 7.25, Start: 5, Sweep: -1.5},
	}
	for _, a := range arcs {
		for _, w := range []float32{1.5, 2} {
			c := shapeCase{
				name: fmt.Sprintf("arc%v/%v", a, w),
				quad: func(o *op.Ops) clip.Op { return clip.Stroke{Path: a.Path(o), Width: w}.Op() },
				path: func(o *op.Ops) clip.Op { return clip.Stroke{Path: arcPath(o, a), Width: w}.Op() },
			}
			if a.Sweep >= 2*math.Pi {
				c.overlaps = 1
			}
			cases = append(cases, c)
		}
	}
	rr := rrects[1]
	cases = append(cases, shapeCase{
		name:  "layer",
		quad:  func(o *op.Ops) clip.Op { return clip.Stroke{Path: rr.Path(o), Width: 1}.Op() },
		path:  func(o *op.Ops) clip.Op { return clip.Stroke{Path: rrectPath(o, rr), Width: 1}.Op() },
		layer: true,
	})
	return cases
}

func drawShape(t *testing.T, s float32, layer bool, shape func(o *op.Ops) clip.Op) *image.RGBA {
	t.Helper()
	img, err := drawImage(t, shapeWindow, new(op.Ops), func(o *op.Ops) {
		defer op.Affine(f32.AffineId().Scale(f32.Point{}, f32.Pt(s, s)).Offset(f32.Pt(0.25, 0))).Push(o).Pop()
		if layer {
			defer paint.PushOpacity(o, 0.6).Pop()
		}
		paint.FillShape(o, color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0xff}, shape(o))
	})
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func sharpCorners(rr clip.RRect) int {
	n := 0
	for _, r := range []int{rr.SE, rr.SW, rr.NW, rr.NE} {
		if r == 0 {
			n++
		}
	}
	return n
}

// coverageDiff compares two pixels of an opaque color drawn on nothing,
// whose alpha is the coverage.
func coverageDiff(a, b color.RGBA) int {
	return int(math.Abs(float64(int(a.A) - int(b.A))))
}

// rrectPath is clip.RRect.Path's outline without its shape: a path the
// renderer has to stencil.
func rrectPath(o *op.Ops, rr clip.RRect) clip.PathSpec {
	const q = 4 * (math.Sqrt2 - 1) / 3
	const iq = 1 - q
	se, sw, nw, ne := float32(rr.SE), float32(rr.SW), float32(rr.NW), float32(rr.NE)
	w, n, e, s := float32(rr.Rect.Min.X), float32(rr.Rect.Min.Y), float32(rr.Rect.Max.X), float32(rr.Rect.Max.Y)
	var p clip.Path
	p.Begin(o)
	p.MoveTo(f32.Pt(w+nw, n))
	p.LineTo(f32.Pt(e-ne, n))
	p.CubeTo(f32.Pt(e-ne*iq, n), f32.Pt(e, n+ne*iq), f32.Pt(e, n+ne))
	p.LineTo(f32.Pt(e, s-se))
	p.CubeTo(f32.Pt(e, s-se*iq), f32.Pt(e-se*iq, s), f32.Pt(e-se, s))
	p.LineTo(f32.Pt(w+sw, s))
	p.CubeTo(f32.Pt(w+sw*iq, s), f32.Pt(w, s-sw*iq), f32.Pt(w, s-sw))
	p.LineTo(f32.Pt(w, n+nw))
	p.CubeTo(f32.Pt(w, n+nw*iq), f32.Pt(w+nw*iq, n), f32.Pt(w+nw, n))
	return p.End()
}

// arcPath is clip.Arc.Path without its shape.
func arcPath(o *op.Ops, a clip.Arc) clip.PathSpec {
	var p clip.Path
	p.Begin(o)
	sin, cos := math.Sincos(float64(a.Start))
	p.MoveTo(a.Center.Add(f32.Pt(float32(cos), float32(sin)).Mul(a.Radius)))
	p.ArcTo(a.Center, a.Center, a.Sweep)
	return p.End()
}
