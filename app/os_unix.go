// SPDX-License-Identifier: Unlicense OR MIT

//go:build (linux && !android) || freebsd || openbsd

package app

import (
	"errors"
	"unsafe"

	"gioui.org/io/pointer"
)

// passwordManagerHintType is the clipboard type that marks sensitive content
// for clipboard managers, with the data passwordManagerHintValue. It is the
// convention of KDE's Klipper, which then keeps no history entry.
const (
	passwordManagerHintType  = "x-kde-passwordManagerHint"
	passwordManagerHintValue = "secret"
)

type X11ViewEvent struct {
	// Display is a pointer to the X11 Display created by XOpenDisplay.
	Display unsafe.Pointer
	// Window is the X11 window ID as returned by XCreateWindow.
	Window uintptr
}

func (X11ViewEvent) implementsViewEvent() {}
func (X11ViewEvent) ImplementsEvent()     {}
func (x X11ViewEvent) Valid() bool {
	return x != (X11ViewEvent{})
}

type WaylandViewEvent struct {
	// Display is the *wl_display returned by wl_display_connect.
	Display unsafe.Pointer
	// Surface is the *wl_surface returned by wl_compositor_create_surface.
	Surface unsafe.Pointer
	// Toplevel is the *xdg_toplevel returned by xdg_surface_get_toplevel.
	Toplevel unsafe.Pointer
}

func (WaylandViewEvent) implementsViewEvent() {}
func (WaylandViewEvent) ImplementsEvent()     {}
func (w WaylandViewEvent) Valid() bool {
	return w != (WaylandViewEvent{})
}

func osMain() {
	select {}
}

type windowDriver func(*callbacks, []Option) error

// Instead of creating files with build tags for each combination of wayland +/- x11
// let each driver initialize these variables with their own version of createWindow.
var wlDriver, x11Driver windowDriver

// x11First is set by PreferX11.
var x11First bool

// PreferX11 makes the windows opened after it try X11 before Wayland, so a
// program that places native X11 windows of its own inside its window, such
// as a web view, gets an X11 window where XWayland runs. Wayland stays the
// fallback where X11 cannot open.
func PreferX11() {
	x11First = true
}

func newWindow(window *callbacks, options []Option) {
	var errFirst error
	drivers := []windowDriver{wlDriver, x11Driver}
	if x11First {
		drivers = []windowDriver{x11Driver, wlDriver}
	}
	for _, d := range drivers {
		if d == nil {
			continue
		}
		err := d(window, options)
		if err == nil {
			return
		}
		if errFirst == nil {
			errFirst = err
		}
	}
	if errFirst == nil {
		errFirst = errors.New("app: no window driver available")
	}
	window.ProcessEvent(DestroyEvent{Err: errFirst})
}

// xCursor contains mapping from pointer.Cursor to XCursor.
var xCursor = [...]string{
	pointer.CursorDefault:                  "left_ptr",
	pointer.CursorNone:                     "",
	pointer.CursorText:                     "xterm",
	pointer.CursorVerticalText:             "vertical-text",
	pointer.CursorPointer:                  "hand2",
	pointer.CursorCrosshair:                "crosshair",
	pointer.CursorAllScroll:                "fleur",
	pointer.CursorColResize:                "sb_h_double_arrow",
	pointer.CursorRowResize:                "sb_v_double_arrow",
	pointer.CursorGrab:                     "hand1",
	pointer.CursorGrabbing:                 "move",
	pointer.CursorNotAllowed:               "crossed_circle",
	pointer.CursorWait:                     "watch",
	pointer.CursorProgress:                 "left_ptr_watch",
	pointer.CursorNorthWestResize:          "top_left_corner",
	pointer.CursorNorthEastResize:          "top_right_corner",
	pointer.CursorSouthWestResize:          "bottom_left_corner",
	pointer.CursorSouthEastResize:          "bottom_right_corner",
	pointer.CursorNorthSouthResize:         "sb_v_double_arrow",
	pointer.CursorEastWestResize:           "sb_h_double_arrow",
	pointer.CursorWestResize:               "left_side",
	pointer.CursorEastResize:               "right_side",
	pointer.CursorNorthResize:              "top_side",
	pointer.CursorSouthResize:              "bottom_side",
	pointer.CursorNorthEastSouthWestResize: "fd_double_arrow",
	pointer.CursorNorthWestSouthEastResize: "bd_double_arrow",
}
