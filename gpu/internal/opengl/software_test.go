// SPDX-License-Identifier: Unlicense OR MIT

package opengl

import "testing"

func TestSoftwareRenderer(t *testing.T) {
	cases := map[string]bool{
		"llvmpipe (LLVM 19.1.7, 256 bits)":                  true,
		"softpipe":                                          true,
		"Software Rasterizer":                               true,
		"swrast":                                            true,
		"Google SwiftShader":                                true,
		"NVIDIA GeForce RTX 3050 6GB Laptop GPU":            false,
		"Mesa Intel(R) Graphics (ADL GT2)":                  false,
		"AMD Radeon RX 7800 XT (radeonsi, navi32, LLVM 19)": false,
	}
	for name, want := range cases {
		if got := softwareRenderer(name); got != want {
			t.Errorf("softwareRenderer(%q) = %v, want %v", name, got, want)
		}
	}
}
