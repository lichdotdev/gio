// SPDX-License-Identifier: Unlicense OR MIT

//go:build windows && !386

package app

// The IDropTarget methods take a POINTL by value, which the 64-bit
// calling conventions pass in one register, x in the low half.

func dropTargetDragEnter(this *dropTarget, data *iDataObject, keyState, pt uintptr, effect *uint32) uintptr {
	return this.dragEnter(data, int32(pt), int32(pt>>32), effect)
}

func dropTargetDragOver(this *dropTarget, keyState, pt uintptr, effect *uint32) uintptr {
	return this.dragOver(int32(pt), int32(pt>>32), effect)
}

func dropTargetDrop(this *dropTarget, data *iDataObject, keyState, pt uintptr, effect *uint32) uintptr {
	return this.drop(data, int32(pt), int32(pt>>32), effect)
}
