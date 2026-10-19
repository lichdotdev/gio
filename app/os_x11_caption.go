// SPDX-License-Identifier: Unlicense OR MIT

//go:build ((linux && !android) || freebsd || openbsd) && !nox11

package app

/*
#include <stdlib.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>
*/
import "C"

import (
	"strings"
	"time"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/io/system"
)

// The _NET_WM_MOVERESIZE directions, from the EWMH spec, indexed by edge.
var moveResizeDirection = [...]C.long{
	edgeNorthWest: 0,
	edgeNorth:     1,
	edgeNorthEast: 2,
	edgeEast:      3,
	edgeSouthEast: 4,
	edgeSouth:     5,
	edgeSouthWest: 6,
	edgeWest:      7,
}

const (
	moveResizeMove = 8
	// sourceApplication is EWMH's source indication for a normal client.
	sourceApplication = 1
	// motifHintsDecorations is MWM_HINTS_DECORATIONS: only the decorations
	// field of _MOTIF_WM_HINTS is set, to zero, so the WM draws nothing.
	motifHintsDecorations = 1 << 1
	motifHintsLen         = 5
	// captionBorder is how deep, in dp, the resize grab reaches into an
	// undecorated window; Wayland's systemGesture uses the same.
	captionBorder = 3
)

// tilingWMs are window managers, by their _NET_WM_NAME, that tile: they
// place every window themselves, so a client-drawn caption has nothing to
// move or maximize and the WM's own decoration (or none) is kept.
// ponytail: a name list, so a tiling WM missing from it gets the client's
// caption buttons; extend it as reports come in.
var tilingWMs = []string{"i3", "bspwm", "awesome", "xmonad", "lg3d", "herbstluftwm", "qtile", "spectrwm", "leftwm", "dwm", "ratpoison", "stumpwm", "notion"}

func isTilingWM(name string) bool {
	name = strings.ToLower(name)
	for _, wm := range tilingWMs {
		if name == wm || strings.HasPrefix(name, wm+" ") {
			return true
		}
	}
	return false
}

// wmName is the running window manager's _NET_WM_NAME, empty without an
// EWMH window manager.
func (w *x11Window) wmName() string {
	check := w.windowProperty(C.XDefaultRootWindow(w.x), w.atom("_NET_SUPPORTING_WM_CHECK", false), C.XA_WINDOW)
	if len(check) < int(unsafe.Sizeof(C.Window(0))) {
		return ""
	}
	wm := *(*C.Window)(unsafe.Pointer(&check[0]))
	return string(w.windowProperty(wm, w.atoms.wmName, w.atoms.utf8string))
}

// windowProperty reads prop of type typ from win, nil if it is not set.
func (w *x11Window) windowProperty(win C.Window, prop, typ C.Atom) []byte {
	var (
		actual C.Atom
		format C.int
		n      C.ulong
		after  C.ulong
		data   *C.uchar
	)
	const maxLongs = 1024
	if C.XGetWindowProperty(w.x, win, prop, 0, maxLongs, C.False, typ, &actual, &format, &n, &after, &data) != C.Success || data == nil {
		return nil
	}
	defer C.XFree(unsafe.Pointer(data))
	if actual != typ || n == 0 {
		return nil
	}
	size := int(n)
	switch format {
	case 16:
		size *= 2
	case 32:
		// Format 32 items are longs in Xlib's memory, whatever their width.
		size *= int(unsafe.Sizeof(C.long(0)))
	}
	return C.GoBytes(unsafe.Pointer(data), C.int(size))
}

// setMotifDecorations asks the WM to draw its decorations, or none.
func (w *x11Window) setMotifDecorations(decorated bool) {
	prop := w.atom("_MOTIF_WM_HINTS", false)
	if decorated {
		C.XDeleteProperty(w.x, w.xw, prop)
		return
	}
	hints := [motifHintsLen]C.long{motifHintsDecorations}
	C.XChangeProperty(w.x, w.xw, prop, prop, 32, C.PropModeReplace, (*C.uchar)(unsafe.Pointer(&hints[0])), motifHintsLen)
}

// captionPress handles a primary press on an undecorated window: a resize
// from an edge, a move from a move area, or a double click there toggling
// maximization. It reports whether the press was the window's, not the app's.
func (w *x11Window) captionPress(ev *C.XButtonEvent, p f32.Point, t time.Duration) bool {
	if w.config.Decorated || w.config.Mode == Fullscreen {
		return false
	}
	if edge := w.edgeAt(p); edge != edgeNone && w.config.Mode == Windowed {
		w.moveResize(ev, moveResizeDirection[edge])
		return true
	}
	if act, ok := w.w.ActionAt(p); !ok || act != system.ActionMove {
		return false
	}
	if w.caption.press(t, p) {
		w.Configure([]Option{toggledMode(w.config.Mode).Option()})
		return true
	}
	w.moveResize(ev, moveResizeMove)
	return true
}

func (w *x11Window) edgeAt(p f32.Point) captionEdge {
	return edgeAt(p, w.config.Size, w.metric.Dp(captionBorder))
}

// moveResize hands the drag to the WM. The press's implicit grab is released
// first, or the WM cannot take the pointer.
func (w *x11Window) moveResize(ev *C.XButtonEvent, direction C.long) {
	C.XUngrabPointer(w.x, C.CurrentTime)
	var xev C.XEvent
	msg := (*C.XClientMessageEvent)(unsafe.Pointer(&xev))
	*msg = C.XClientMessageEvent{
		_type:        C.ClientMessage,
		display:      w.x,
		window:       w.xw,
		message_type: w.atom("_NET_WM_MOVERESIZE", false),
		format:       32,
	}
	data := (*[5]C.long)(unsafe.Pointer(&msg.data))
	data[0] = C.long(ev.x_root)
	data[1] = C.long(ev.y_root)
	data[2] = direction
	data[3] = C.long(ev.button)
	data[4] = sourceApplication
	C.XSendEvent(w.x, C.XDefaultRootWindow(w.x), C.False, C.SubstructureNotifyMask|C.SubstructureRedirectMask, &xev)
	C.XFlush(w.x)
}

// captionHover shows the resize cursor over an undecorated window's edges,
// and puts the app's cursor back once the pointer leaves them.
func (w *x11Window) captionHover(p f32.Point) {
	edge := edgeNone
	if !w.config.Decorated && w.config.Mode == Windowed {
		edge = w.edgeAt(p)
	}
	if edge == w.hoverEdge {
		return
	}
	w.hoverEdge = edge
	if edge == edgeNone {
		w.SetCursor(w.cursor)
		return
	}
	app := w.cursor
	w.SetCursor(captionEdgeCursor[edge])
	w.cursor = app
}
