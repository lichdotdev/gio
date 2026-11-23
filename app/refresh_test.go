// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"image"
	"testing"

	"gioui.org/gpu"
	"gioui.org/op"
)

// outOfDateContext is a surface whose size the window system has not caught
// up with yet, as XWayland's lags a ConfigureNotify.
type outOfDateContext struct{ refreshes int }

func (c *outOfDateContext) API() gpu.API                            { return nil }
func (c *outOfDateContext) RenderTarget() (gpu.RenderTarget, error) { return nil, nil }
func (c *outOfDateContext) Present() error                          { return nil }
func (c *outOfDateContext) Refresh() error                          { c.refreshes++; return errOutOfDate }
func (c *outOfDateContext) Release()                                {}
func (c *outOfDateContext) Lock() error                             { return nil }
func (c *outOfDateContext) Unlock()                                 {}

func TestOutOfDateRefreshRetries(t *testing.T) {
	ctx := new(outOfDateContext)
	w := &Window{ctx: ctx}
	if err := w.validateAndProcess(image.Pt(800, 600), true, new(op.Ops), nil); err != nil {
		t.Fatal(err)
	}
	if !w.hasNextFrame {
		t.Fatal("a refresh the surface was not ready for asked for no next frame; the window stays unpainted")
	}
	if err := w.validateAndProcess(image.Pt(800, 600), false, new(op.Ops), nil); err != nil {
		t.Fatal(err)
	}
	if ctx.refreshes != 2 {
		t.Fatalf("refreshes = %d, want 2: the next frame must refresh the surface again", ctx.refreshes)
	}
}
