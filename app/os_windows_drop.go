// SPDX-License-Identifier: Unlicense OR MIT

//go:build windows

package app

import (
	"net/url"
	"strings"
	"syscall"
	"unsafe"

	"gioui.org/app/internal/windows"
	"gioui.org/f32"
)

// dropTarget implements the COM IDropTarget interface for a window.
// Its lifetime is the window's, so reference counting is a no-op.
type dropTarget struct {
	vtbl *dropTargetVtbl
	w    *window
	// accept is set when the drag in progress offers files.
	accept bool
}

type dropTargetVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	DragEnter      uintptr
	DragOver       uintptr
	DragLeave      uintptr
	Drop           uintptr
}

// iDataObject is a COM IDataObject.
type iDataObject struct {
	vtbl *struct {
		QueryInterface uintptr
		AddRef         uintptr
		Release        uintptr
		GetData        uintptr
		GetDataHere    uintptr
		QueryGetData   uintptr
	}
}

var (
	iidIUnknown    = syscall.GUID{Data1: 0x00000000, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidIDropTarget = syscall.GUID{Data1: 0x00000122, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
)

const (
	_S_OK          = 0
	_E_NOINTERFACE = 0x80004002
)

var dropTargetVtable = &dropTargetVtbl{
	QueryInterface: syscall.NewCallback(dropTargetQueryInterface),
	AddRef:         syscall.NewCallback(dropTargetAddRef),
	Release:        syscall.NewCallback(dropTargetRelease),
	DragEnter:      syscall.NewCallback(dropTargetDragEnter),
	DragOver:       syscall.NewCallback(dropTargetDragOver),
	DragLeave:      syscall.NewCallback(dropTargetDragLeave),
	Drop:           syscall.NewCallback(dropTargetDrop),
}

var hdropFormat = windows.FormatEtc{
	CfFormat: windows.CF_HDROP,
	DwAspect: windows.DVASPECT_CONTENT,
	Lindex:   -1,
	Tymed:    windows.TYMED_HGLOBAL,
}

// registerDropTarget makes w a target for drags from other programs.
// It must be called from the window thread.
func (w *window) registerDropTarget() error {
	if err := windows.OleInitialize(); err != nil {
		return err
	}
	w.dropTarget = &dropTarget{vtbl: dropTargetVtable, w: w}
	return windows.RegisterDragDrop(w.hwnd, unsafe.Pointer(w.dropTarget))
}

func dropTargetQueryInterface(this *dropTarget, riid *syscall.GUID, ppv *uintptr) uintptr {
	if *riid != iidIUnknown && *riid != iidIDropTarget {
		*ppv = 0
		return _E_NOINTERFACE
	}
	*ppv = uintptr(unsafe.Pointer(this))
	return _S_OK
}

func dropTargetAddRef(this *dropTarget) uintptr {
	return 1
}

func dropTargetRelease(this *dropTarget) uintptr {
	return 1
}

func (t *dropTarget) dragEnter(data *iDataObject, x, y int32, effect *uint32) uintptr {
	f := hdropFormat
	r, _, _ := syscall.SyscallN(data.vtbl.QueryGetData, uintptr(unsafe.Pointer(data)), uintptr(unsafe.Pointer(&f)))
	t.accept = r == _S_OK
	var types []string
	if t.accept {
		types = []string{dropMimeType}
	}
	*effect = t.effect(*effect)
	t.w.ProcessEvent(DropEvent{Kind: DropEnter, Position: t.w.clientPoint(x, y), Types: types})
	return _S_OK
}

func (t *dropTarget) dragOver(x, y int32, effect *uint32) uintptr {
	*effect = t.effect(*effect)
	t.w.ProcessEvent(DropEvent{Kind: DropMove, Position: t.w.clientPoint(x, y)})
	return _S_OK
}

func dropTargetDragLeave(this *dropTarget) uintptr {
	this.accept = false
	this.w.ProcessEvent(DropEvent{Kind: DropLeave})
	return _S_OK
}

func (t *dropTarget) drop(data *iDataObject, x, y int32, effect *uint32) uintptr {
	*effect = t.effect(*effect)
	accept := t.accept
	t.accept = false
	if !accept {
		t.w.ProcessEvent(DropEvent{Kind: DropLeave})
		return _S_OK
	}
	f := hdropFormat
	var m windows.StgMedium
	r, _, _ := syscall.SyscallN(data.vtbl.GetData, uintptr(unsafe.Pointer(data)), uintptr(unsafe.Pointer(&f)), uintptr(unsafe.Pointer(&m)))
	if r != _S_OK {
		*effect = windows.DROPEFFECT_NONE
		t.w.ProcessEvent(DropEvent{Kind: DropLeave})
		return _S_OK
	}
	paths := windows.DragQueryFiles(m.Handle)
	windows.ReleaseStgMedium(&m)
	t.w.ProcessEvent(DropEvent{
		Kind:     DropDrop,
		Position: t.w.clientPoint(x, y),
		Types:    []string{dropMimeType},
		Type:     dropMimeType,
		Data:     fileURIList(paths),
	})
	return _S_OK
}

// effect returns the drop effect to report to the source, given the
// effects it allows.
func (t *dropTarget) effect(allowed uint32) uint32 {
	if t.accept && allowed&windows.DROPEFFECT_COPY != 0 {
		return windows.DROPEFFECT_COPY
	}
	return windows.DROPEFFECT_NONE
}

// clientPoint converts screen coordinates to window pixels.
func (w *window) clientPoint(x, y int32) f32.Point {
	p := windows.Point{X: x, Y: y}
	windows.ScreenToClient(w.hwnd, &p)
	return f32.Point{X: float32(p.X), Y: float32(p.Y)}
}

// fileURIList formats Windows file paths as a text/uri-list.
func fileURIList(paths []string) []byte {
	var b strings.Builder
	for _, p := range paths {
		u := url.URL{Scheme: "file"}
		p = strings.ReplaceAll(p, `\`, "/")
		if strings.HasPrefix(p, "//") {
			// A UNC path, //host/share/file.
			u.Host, u.Path, _ = strings.Cut(p[2:], "/")
			u.Path = "/" + u.Path
		} else {
			u.Path = "/" + p
		}
		b.WriteString(u.String())
		b.WriteString("\r\n")
	}
	return []byte(b.String())
}
