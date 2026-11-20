// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"gioui.org/internal/ops"
	"gioui.org/op"
)

// KeepOp marks the input handlers added while it is on the stack as ones a
// frame may replay from an earlier frame without running their widgets, as
// a cache of a subtree's operations does. Such a handler that asks for no
// events in a frame keeps routing by the filter it last asked for, instead
// of losing it a frame later; outside a KeepOp, a handler routes only what
// its widget asked for in the last frame. Lay the subtree out again on a
// frame with input (see [Source.Queued]) so its widgets take the input.
type KeepOp struct{}

// KeepStack represents a KeepOp on the keep stack.
type KeepStack struct {
	ops     *ops.Ops
	id      ops.StackID
	macroID uint32
}

// Push the KeepOp on the keep stack.
func (KeepOp) Push(o *op.Ops) KeepStack {
	id, mid := ops.PushOp(&o.Internal, ops.KeepStack)
	data := ops.Write(&o.Internal, ops.TypeKeepLen)
	data[0] = byte(ops.TypeKeep)
	return KeepStack{ops: &o.Internal, id: id, macroID: mid}
}

// Pop the KeepOp off the keep stack.
func (s KeepStack) Pop() {
	ops.PopOp(s.ops, ops.KeepStack, s.id, s.macroID)
	data := ops.Write(s.ops, ops.TypePopKeepLen)
	data[0] = byte(ops.TypePopKeep)
}
