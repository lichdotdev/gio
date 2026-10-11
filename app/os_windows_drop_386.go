// SPDX-License-Identifier: Unlicense OR MIT

//go:build windows

package app

// The IDropTarget methods take a POINTL by value, which the 386
// stdcall convention passes as two stack words.

func dropTargetDragEnter(this *dropTarget, data *iDataObject, keyState, x, y uintptr, effect *uint32) uintptr {
	return this.dragEnter(data, int32(x), int32(y), effect)
}

func dropTargetDragOver(this *dropTarget, keyState, x, y uintptr, effect *uint32) uintptr {
	return this.dragOver(int32(x), int32(y), effect)
}

func dropTargetDrop(this *dropTarget, data *iDataObject, keyState, x, y uintptr, effect *uint32) uintptr {
	return this.drop(data, int32(x), int32(y), effect)
}
