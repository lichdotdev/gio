// SPDX-License-Identifier: Unlicense OR MIT

//go:build (linux && !android) || freebsd || openbsd

package app

import (
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/io/pointer"
)

// GTK's defaults for gtk-double-click-time and gtk-double-click-distance, the
// values a GNOME title bar answers to.
const (
	captionDoubleClickTime     = 400 * time.Millisecond
	captionDoubleClickDistance = 5
)

// captionClicks tells a double click on a move area, which toggles
// maximization instead of starting a move, from the press before it.
type captionClicks struct {
	last    time.Duration
	lastPos f32.Point
	primed  bool
}

// press records a primary press on a move area at t and p, and reports
// whether it completes a double click.
func (c *captionClicks) press(t time.Duration, p f32.Point) bool {
	d := p.Sub(c.lastPos)
	double := c.primed && t-c.last <= captionDoubleClickTime &&
		d.X*d.X+d.Y*d.Y <= captionDoubleClickDistance*captionDoubleClickDistance
	c.last, c.lastPos, c.primed = t, p, !double
	return double
}

// toggledMode is the mode a double click on the caption asks for.
func toggledMode(m WindowMode) WindowMode {
	if m == Maximized {
		return Windowed
	}
	return Maximized
}

// captionEdge is the window edge a resize grabs from, compass-named.
type captionEdge int

const (
	edgeNone captionEdge = iota
	edgeNorthWest
	edgeNorth
	edgeNorthEast
	edgeEast
	edgeSouthEast
	edgeSouth
	edgeSouthWest
	edgeWest
)

// edgeAt is the edge under p in a window of size, border pixels deep.
func edgeAt(p f32.Point, size image.Point, border int) captionEdge {
	x, y := int(p.X), int(p.Y)
	north, south := y <= border, y >= size.Y-border
	west, east := x <= border, x >= size.X-border
	switch {
	case north && west:
		return edgeNorthWest
	case north && east:
		return edgeNorthEast
	case south && west:
		return edgeSouthWest
	case south && east:
		return edgeSouthEast
	case north:
		return edgeNorth
	case south:
		return edgeSouth
	case west:
		return edgeWest
	case east:
		return edgeEast
	}
	return edgeNone
}

var captionEdgeCursor = [...]pointer.Cursor{
	edgeNorthWest: pointer.CursorNorthWestResize,
	edgeNorth:     pointer.CursorNorthResize,
	edgeNorthEast: pointer.CursorNorthEastResize,
	edgeEast:      pointer.CursorEastResize,
	edgeSouthEast: pointer.CursorSouthEastResize,
	edgeSouth:     pointer.CursorSouthResize,
	edgeSouthWest: pointer.CursorSouthWestResize,
	edgeWest:      pointer.CursorWestResize,
}
