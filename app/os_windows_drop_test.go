// SPDX-License-Identifier: Unlicense OR MIT

//go:build windows

package app

import "testing"

func TestFileURIList(t *testing.T) {
	got := string(fileURIList([]string{
		`C:\Users\a b\c.txt`,
		`C:\100%.txt`,
		`\\server\share\d e.png`,
	}))
	want := "file:///C:/Users/a%20b/c.txt\r\n" +
		"file:///C:/100%25.txt\r\n" +
		"file://server/share/d%20e.png\r\n"
	if got != want {
		t.Errorf("fileURIList = %q, want %q", got, want)
	}
}
