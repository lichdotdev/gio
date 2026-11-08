// SPDX-License-Identifier: Unlicense OR MIT

package headless

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math/rand/v2"
	"testing"

	"gioui.org/f32"
	"gioui.org/gpu"
	"gioui.org/gpu/internal/driver"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

const (
	testW, testH = 320, 240
	testSeeds    = 12
	testSteps    = 30
	dropEvery    = 11
)

var testClears = []color.NRGBA{
	{R: 30, G: 80, B: 200, A: 255},
	{},
	{R: 20, G: 40, B: 200, A: 77},
}

// item is one thing a test scene draws; change perturbs it a little.
type item struct {
	kind    int
	r       image.Rectangle
	c1, c2  color.NRGBA
	opacity float32
	img     paint.ImageOp
}

const (
	itemRect = iota
	itemGradient
	itemImage
	itemLayerRect
	pathlessKinds

	itemRRect = iota - 1
	itemStroke
	itemLayerPath
	allKinds
)

// pathJitter is how far one channel of a pixel covered by a stencilled path
// can move by where the frame packs that path in the atlas, measured by
// drawing the same scene whole with its paths at other atlas positions:
// anti-aliased coverage rounds differently, by at most one level.
const pathJitter = 1

type scene struct {
	rnd    *rand.Rand
	canvas *color.NRGBA
	items  []item
}

func newScene(seed uint64, kinds int) *scene {
	s := &scene{rnd: rand.New(rand.NewPCG(seed, 2))}
	if s.rnd.IntN(2) == 0 {
		s.canvas = &color.NRGBA{R: 240, G: 230, B: 210, A: 255}
	}
	im := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for i := range im.Pix {
		im.Pix[i] = byte(s.rnd.IntN(256))
	}
	img := paint.NewImageOp(im)
	for range 40 {
		s.items = append(s.items, item{kind: s.rnd.IntN(kinds), r: s.rect(), c1: s.color(), c2: s.color(), opacity: s.rnd.Float32(), img: img})
	}
	return s
}

func (s *scene) rect() image.Rectangle {
	x, y := s.rnd.IntN(testW-20), s.rnd.IntN(testH-20)
	return image.Rect(x, y, x+10+s.rnd.IntN(60), y+10+s.rnd.IntN(60))
}

func (s *scene) color() color.NRGBA {
	return color.NRGBA{R: byte(s.rnd.IntN(256)), G: byte(s.rnd.IntN(256)), B: byte(s.rnd.IntN(256)), A: byte(s.rnd.IntN(256))}
}

// change moves or recolors a few items.
func (s *scene) change() {
	for range 1 + s.rnd.IntN(3) {
		it := &s.items[s.rnd.IntN(len(s.items))]
		switch s.rnd.IntN(4) {
		case 0:
			it.r = s.rect()
		case 1:
			it.c1 = s.color()
		case 2:
			it.opacity = s.rnd.Float32()
		default:
			it.r = it.r.Add(image.Pt(s.rnd.IntN(5)-2, s.rnd.IntN(5)-2))
		}
	}
}

func (s *scene) build(o *op.Ops) {
	o.Reset()
	if s.canvas != nil {
		paint.Fill(o, *s.canvas)
	}
	for i := range s.items {
		s.items[i].draw(o)
	}
}

func (it *item) draw(o *op.Ops) {
	switch it.kind {
	case itemRect:
		paint.FillShape(o, it.c1, clip.Rect(it.r).Op())
	case itemRRect:
		paint.FillShape(o, it.c1, clip.RRect{Rect: it.r, SE: 6, SW: 6, NE: 6, NW: 6}.Op(o))
	case itemStroke:
		paint.FillShape(o, it.c1, clip.Stroke{Path: clip.Ellipse(it.r).Path(o), Width: 2}.Op())
	case itemGradient:
		defer clip.Rect(it.r).Push(o).Pop()
		paint.LinearGradientOp{Stop1: f32.Pt(float32(it.r.Min.X), 0), Color1: it.c1, Stop2: f32.Pt(float32(it.r.Max.X), 0), Color2: it.c2}.Add(o)
		paint.PaintOp{}.Add(o)
	case itemImage:
		defer op.Offset(it.r.Min).Push(o).Pop()
		it.img.Add(o)
		paint.PaintOp{}.Add(o)
	case itemLayerRect:
		defer paint.PushOpacity(o, it.opacity).Pop()
		paint.FillShape(o, it.c1, clip.Rect(it.r).Op())
		paint.FillShape(o, it.c2, clip.Rect(it.r.Inset(4)).Op())
	case itemLayerPath:
		defer paint.PushOpacity(o, it.opacity).Pop()
		paint.FillShape(o, it.c1, clip.Rect(it.r).Op())
		paint.FillShape(o, it.c2, clip.RRect{Rect: it.r.Inset(4), SE: 3, SW: 3, NE: 3, NW: 3}.Op(o))
	}
}

func frame(w *Window, clearColor color.NRGBA, o *op.Ops, size image.Point) error {
	return contextDo(w.ctx, func() error {
		w.gpu.Clear(clearColor)
		return w.gpu.Frame(o, w.fboTex, size)
	})
}

func shot(t *testing.T, w *Window) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rectangle{Max: w.Size()})
	if err := w.Screenshot(img); err != nil {
		t.Fatal(err)
	}
	return img.Pix
}

func damageOf(w *Window) []image.Rectangle {
	return w.gpu.(interface{ Damage() []image.Rectangle }).Damage()
}

// checkPartial draws random scenes changing a little at a time into the
// retained framebuffer, each frame only where it changed, and has compare
// judge each one against the same frame drawn whole, over an opaque clear
// and a translucent one. Now and then the retained contents are dropped by
// a resize, which must redraw whole.
func checkPartial(t *testing.T, kinds int, compare func(got, want []byte) string) {
	t.Setenv("GIO_DAMAGE", "on")
	partial, total := 0, 0
	for seed := range uint64(testSeeds) {
		w, whole := openWindow(t), openDirectWindow(t)
		sc := newScene(seed, kinds)
		clearColor := testClears[seed%uint64(len(testClears))]
		ops := new(op.Ops)
		for step := range testSteps {
			if step > 0 {
				sc.change()
			}
			sc.build(ops)
			dropped := step%dropEvery == dropEvery-1
			if dropped {
				if err := frame(w, clearColor, ops, w.Size().Sub(image.Pt(5, 5))); err != nil {
					t.Fatal(err)
				}
			}
			if err := frame(w, clearColor, ops, w.Size()); err != nil {
				t.Fatal(err)
			}
			if err := frame(whole, clearColor, ops, whole.Size()); err != nil {
				t.Fatal(err)
			}
			got, want := shot(t, w), shot(t, whole)
			region := damageOf(w)
			isWhole := len(region) == 1 && region[0] == (image.Rectangle{Max: w.Size()})
			if dropped && !isWhole {
				t.Fatalf("seed %d step %d: dropped contents drew only %v", seed, step, region)
			}
			if msg := compare(got, want); msg != "" {
				t.Fatalf("seed %d step %d (whole %v, dropped %v, region %v): %s", seed, step, isWhole, dropped, region, msg)
			}
			total++
			if !isWhole {
				partial++
			}
		}
		w.Release()
		whole.Release()
	}
	if partial < total/3 {
		t.Errorf("only %d of %d frames were drawn in part", partial, total)
	}
}

// TestPartialFramesMatchWhole holds a frame drawn in part to the bytes of
// the frame drawn whole. Its scenes have no stencilled paths: a path's
// coverage rounds by its atlas position, which differs between any two
// frames, so whole frames do not match each other byte for byte there
// either. TestPartialPathFramesMatchWhole covers those.
func TestPartialFramesMatchWhole(t *testing.T) {
	checkPartial(t, pathlessKinds, func(got, want []byte) string {
		if bytes.Equal(got, want) {
			return ""
		}
		return fmt.Sprintf("%d bytes differ from the frame drawn whole", diffCount(got, want))
	})
}

func TestPartialPathFramesMatchWhole(t *testing.T) {
	checkPartial(t, allKinds, func(got, want []byte) string {
		for i := range got {
			if d := int(got[i]) - int(want[i]); d > pathJitter || d < -pathJitter {
				return fmt.Sprintf("byte %d is %d, the frame drawn whole %d", i, got[i], want[i])
			}
		}
		return ""
	})
}

func diffCount(a, b []byte) int {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return n
}

func openWindow(t *testing.T) *Window {
	t.Helper()
	w, err := NewWindow(testW, testH)
	if err != nil {
		t.Skipf("headless windows not supported: %v", err)
	}
	return w
}

// directDevice hides the blit from gpu, which then draws every frame whole
// and straight to the output: what the retained framebuffer has to match.
type directDevice struct{ driver.Device }

func (d directDevice) Caps() driver.Caps {
	c := d.Device.Caps()
	c.Features &^= driver.FeatureBlitFramebuffer
	return c
}

func openDirectWindow(t *testing.T) *Window {
	t.Helper()
	ctx, err := newContext()
	if err != nil {
		t.Skipf("headless windows not supported: %v", err)
	}
	w := &Window{size: image.Pt(testW, testH), ctx: ctx}
	err = contextDo(ctx, func() error {
		dev, err := driver.NewDevice(ctx.API())
		if err != nil {
			return err
		}
		w.fboTex, err = dev.NewTexture(driver.TextureFormatSRGBA, testW, testH, driver.FilterNearest, driver.FilterNearest, driver.BufferBindingFramebuffer)
		if err != nil {
			dev.Release()
			return err
		}
		w.dev = dev
		w.gpu, err = gpu.NewWithDevice(directDevice{dev})
		return err
	})
	if err != nil {
		ctx.Release()
		t.Fatal(err)
	}
	return w
}
