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
	"slices"
	"unsafe"

	"gioui.org/f32"
)

// The XDND protocol is specified at
// https://freedesktop.org/wiki/Specifications/XDND/.

// xdndVersion is the protocol version advertised in XdndAware.
const xdndVersion = 5

// xdndMaxDataLen bounds the dropped data read in one request, in
// 32-bit units. Larger transfers use the INCR mechanism, which is
// not supported.
const xdndMaxDataLen = 1 << 24

type xdndAtoms struct {
	aware, enter, position, status, leave, drop, finished C.Atom
	selection, typeList, actionCopy, uriList              C.Atom
}

// xdndState tracks a drag from another program.
type xdndState struct {
	// source is the window of the drag source, or None when no
	// drag is over the window.
	source  C.Window
	version int
	types   []string
	accept  bool
	// entered is set once DropEnter is delivered, at the first
	// XdndPosition.
	entered bool
	pos     f32.Point
	// dropping is set when the data of a drop has been
	// requested and not yet received.
	dropping bool
}

func (w *x11Window) initXdnd() {
	a := &w.xdndAtoms
	a.aware = w.atom("XdndAware", false)
	a.enter = w.atom("XdndEnter", false)
	a.position = w.atom("XdndPosition", false)
	a.status = w.atom("XdndStatus", false)
	a.leave = w.atom("XdndLeave", false)
	a.drop = w.atom("XdndDrop", false)
	a.finished = w.atom("XdndFinished", false)
	a.selection = w.atom("XdndSelection", false)
	a.typeList = w.atom("XdndTypeList", false)
	a.actionCopy = w.atom("XdndActionCopy", false)
	a.uriList = w.atom(dropMimeType, false)
	version := C.long(xdndVersion)
	C.XChangeProperty(w.x, w.xw, a.aware, C.XA_ATOM, 32, C.PropModeReplace,
		(*C.uchar)(unsafe.Pointer(&version)), 1)
}

// handleXdndMessage handles an XDND client message and reports
// whether it was one.
func (w *x11Window) handleXdndMessage(cevt *C.XClientMessageEvent) bool {
	a := &w.xdndAtoms
	d := &w.xdnd
	l := (*[5]C.long)(unsafe.Pointer(&cevt.data))
	src := C.Window(l[0])
	switch cevt.message_type {
	case a.enter:
		version := int(uint64(l[1]) >> 24)
		if version > xdndVersion {
			return true
		}
		var types []string
		if l[1]&1 != 0 {
			types = w.xdndTypeList(src)
		} else {
			for _, t := range l[2:] {
				if t != C.None {
					types = append(types, w.atomName(C.Atom(t)))
				}
			}
		}
		*d = xdndState{
			source:  src,
			version: version,
			types:   types,
			accept:  slices.Contains(types, dropMimeType),
		}
	case a.position:
		if src != d.source || d.dropping {
			return true
		}
		var x, y C.int
		var child C.Window
		rootX, rootY := C.int(l[2]>>16), C.int(l[2]&0xffff)
		C.XTranslateCoordinates(w.x, C.XDefaultRootWindow(w.x), w.xw, rootX, rootY, &x, &y, &child)
		pos := f32.Point{X: float32(x), Y: float32(y)}
		d.pos = pos
		if d.entered {
			w.ProcessEvent(DropEvent{Kind: DropMove, Position: pos})
		} else {
			d.entered = true
			w.ProcessEvent(DropEvent{Kind: DropEnter, Position: pos, Types: d.types})
		}
		status := [5]C.long{C.long(w.xw)}
		if d.accept {
			// Accept, and ask for positions throughout the window.
			status[1] = 1 | 2
			status[4] = C.long(a.actionCopy)
		}
		w.sendXdnd(src, a.status, status)
	case a.leave:
		if src != d.source || d.dropping {
			return true
		}
		w.endXdnd()
	case a.drop:
		if src != d.source || d.dropping {
			return true
		}
		if !d.accept {
			w.sendXdnd(src, a.finished, [5]C.long{C.long(w.xw)})
			w.endXdnd()
			return true
		}
		d.dropping = true
		var t C.Time = C.CurrentTime
		if d.version >= 1 {
			t = C.Time(l[2])
		}
		C.XConvertSelection(w.x, a.selection, a.uriList, a.selection, w.xw, t)
	default:
		return false
	}
	return true
}

// handleXdndSelection delivers the dropped data converted by XdndDrop.
func (w *x11Window) handleXdndSelection(cevt *C.XSelectionEvent) {
	a := &w.xdndAtoms
	d := &w.xdnd
	if !d.dropping {
		return
	}
	src := d.source
	if cevt.property == C.None {
		w.sendXdnd(src, a.finished, [5]C.long{C.long(w.xw)})
		w.endXdnd()
		return
	}
	var (
		typ        C.Atom
		format     C.int
		n, rem     C.ulong
		value      *C.uchar
		dataLength = C.long(xdndMaxDataLen)
	)
	if C.XGetWindowProperty(w.x, w.xw, a.selection, 0, dataLength, C.True, C.AnyPropertyType,
		&typ, &format, &n, &rem, &value) != C.Success || format != 8 || typ == w.atom("INCR", false) {
		if value != nil {
			C.XFree(unsafe.Pointer(value))
		}
		w.sendXdnd(src, a.finished, [5]C.long{C.long(w.xw)})
		w.endXdnd()
		return
	}
	data := C.GoBytes(unsafe.Pointer(value), C.int(n))
	C.XFree(unsafe.Pointer(value))
	w.sendXdnd(src, a.finished, [5]C.long{C.long(w.xw), 1, C.long(a.actionCopy)})
	e := DropEvent{Kind: DropDrop, Position: d.pos, Types: d.types, Type: dropMimeType, Data: data}
	w.xdnd = xdndState{}
	w.ProcessEvent(e)
}

// endXdnd ends the current drag without a drop.
func (w *x11Window) endXdnd() {
	entered := w.xdnd.entered
	w.xdnd = xdndState{}
	if entered {
		w.ProcessEvent(DropEvent{Kind: DropLeave})
	}
}

func (w *x11Window) xdndTypeList(src C.Window) []string {
	var (
		typ    C.Atom
		format C.int
		n, rem C.ulong
		value  *C.uchar
	)
	const maxTypes = 1024
	if C.XGetWindowProperty(w.x, src, w.xdndAtoms.typeList, 0, maxTypes, C.False, C.XA_ATOM,
		&typ, &format, &n, &rem, &value) != C.Success || value == nil {
		return nil
	}
	defer C.XFree(unsafe.Pointer(value))
	if format != 32 {
		return nil
	}
	// Xlib returns 32-bit properties as an array of longs.
	atoms := unsafe.Slice((*C.long)(unsafe.Pointer(value)), n)
	types := make([]string, 0, len(atoms))
	for _, t := range atoms {
		types = append(types, w.atomName(C.Atom(t)))
	}
	return types
}

func (w *x11Window) atomName(a C.Atom) string {
	cname := C.XGetAtomName(w.x, a)
	if cname == nil {
		return ""
	}
	defer C.XFree(unsafe.Pointer(cname))
	return C.GoString(cname)
}

func (w *x11Window) sendXdnd(target C.Window, msg C.Atom, data [5]C.long) {
	var xev C.XEvent
	ev := (*C.XClientMessageEvent)(unsafe.Pointer(&xev))
	*ev = C.XClientMessageEvent{
		_type:        C.ClientMessage,
		display:      w.x,
		window:       target,
		message_type: msg,
		format:       32,
	}
	*(*[5]C.long)(unsafe.Pointer(&ev.data)) = data
	C.XSendEvent(w.x, target, C.False, C.NoEventMask, &xev)
}
