// SPDX-License-Identifier: Unlicense OR MIT

//go:build (linux && !android) || freebsd || openbsd

package app

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
)

func TestCaptionDoubleClick(t *testing.T) {
	ms := time.Millisecond
	cases := []struct {
		name   string
		second time.Duration
		pos    f32.Point
		want   bool
	}{
		{"quick and still", 200 * ms, f32.Pt(10, 10), true},
		{"too slow", 401 * ms, f32.Pt(10, 10), false},
		{"moved too far", 200 * ms, f32.Pt(16, 10), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var clicks captionClicks
			if clicks.press(0, f32.Pt(10, 10)) {
				t.Fatal("a first press is a double click")
			}
			if got := clicks.press(c.second, c.pos); got != c.want {
				t.Errorf("second press: got %v, want %v", got, c.want)
			}
		})
	}
}

func TestCaptionTripleClickIsNotTwoDoubles(t *testing.T) {
	var clicks captionClicks
	clicks.press(0, f32.Pt(0, 0))
	if !clicks.press(100*time.Millisecond, f32.Pt(0, 0)) {
		t.Fatal("second press: want a double click")
	}
	if clicks.press(200*time.Millisecond, f32.Pt(0, 0)) {
		t.Error("third press: a double click toggled twice")
	}
}

func TestToggledMode(t *testing.T) {
	if got := toggledMode(Maximized); got != Windowed {
		t.Errorf("from Maximized: got %v", got)
	}
	if got := toggledMode(Windowed); got != Maximized {
		t.Errorf("from Windowed: got %v", got)
	}
}

func TestEdgeAt(t *testing.T) {
	size := image.Pt(100, 80)
	cases := []struct {
		p    f32.Point
		want captionEdge
	}{
		{f32.Pt(50, 40), edgeNone},
		{f32.Pt(1, 1), edgeNorthWest},
		{f32.Pt(50, 2), edgeNorth},
		{f32.Pt(99, 0), edgeNorthEast},
		{f32.Pt(98, 40), edgeEast},
		{f32.Pt(99, 79), edgeSouthEast},
		{f32.Pt(50, 78), edgeSouth},
		{f32.Pt(0, 79), edgeSouthWest},
		{f32.Pt(3, 40), edgeWest},
		{f32.Pt(4, 40), edgeNone},
	}
	for _, c := range cases {
		if got := edgeAt(c.p, size, 3); got != c.want {
			t.Errorf("edgeAt(%v) = %v, want %v", c.p, got, c.want)
		}
	}
}
