// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"gioui.org/f32"
)

// DropEvent is sent when data from outside the program, such as files
// from a file manager, is dragged over a window and possibly dropped on
// it. A drag over a window is a DropEnter followed by any number of
// DropMove and ends with exactly one DropLeave or DropDrop.
//
// Only text/uri-list data is requested from the drag source. A drag that
// doesn't offer it still produces DropEnter, DropMove and DropLeave, but
// the platform is told the window won't accept it.
//
// DropEvents are supported on Wayland, X11 and Windows.
type DropEvent struct {
	Kind DropKind
	// Position is the drag position in pixels, in the coordinate
	// space of FrameEvent. It is unset for DropLeave.
	Position f32.Point
	// Types lists the MIME types offered by the drag source. It is
	// set for DropEnter and DropDrop. On Windows, it lists only
	// text/uri-list, when the source offers files.
	Types []string
	// Type is the MIME type of Data. It is set only for DropDrop.
	Type string
	// Data is the dropped data, read in full before the event is
	// delivered. For the text/uri-list type it is a list of
	// CRLF-separated URIs as specified by RFC 2483. It is set
	// only for DropDrop.
	Data []byte
}

// dropMimeType is the only type requested from drags from outside
// the program.
const dropMimeType = "text/uri-list"

// DropKind is the kind of a DropEvent.
type DropKind uint8

const (
	// DropEnter is sent when a drag enters the window.
	DropEnter DropKind = iota
	// DropMove is sent when a drag moves within the window.
	DropMove
	// DropLeave is sent when a drag leaves the window or is
	// cancelled.
	DropLeave
	// DropDrop is sent when data is dropped on the window.
	DropDrop
)

func (DropEvent) ImplementsEvent() {}

func (k DropKind) String() string {
	switch k {
	case DropEnter:
		return "DropEnter"
	case DropMove:
		return "DropMove"
	case DropLeave:
		return "DropLeave"
	case DropDrop:
		return "DropDrop"
	default:
		panic("invalid DropKind")
	}
}
