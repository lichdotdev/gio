// SPDX-License-Identifier: Unlicense OR MIT

//go:build (linux || freebsd) && !novulkan

package vulkan

import "testing"

func TestPooledSizeHoldsTheBytesInAPowerOfTwo(t *testing.T) {
	for _, c := range []struct{ size, want int }{
		{0, 256}, {1, 256}, {256, 256}, {257, 512}, {4000, 4096}, {4096, 4096}, {70000, 131072},
	} {
		if got := pooledSize(c.size); got != c.want {
			t.Errorf("pooledSize(%d) = %d, want %d", c.size, got, c.want)
		}
	}
}
