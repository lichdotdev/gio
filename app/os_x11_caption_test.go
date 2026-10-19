// SPDX-License-Identifier: Unlicense OR MIT

//go:build ((linux && !android) || freebsd || openbsd) && !nox11

package app

import "testing"

func TestIsTilingWM(t *testing.T) {
	for name, want := range map[string]bool{
		"i3": true, "bspwm": true, "LG3D": true, "awesome v4.3": true,
		"GNOME Shell": false, "KWin": false, "Mutter (Muffin)": false, "": false, "i3bar-ish": false,
	} {
		if got := isTilingWM(name); got != want {
			t.Errorf("isTilingWM(%q) = %v, want %v", name, got, want)
		}
	}
}
