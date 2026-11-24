// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"image"
	"image/color"
	"math"
)

// The icon is drawn from shapes on a 256x256 grid: a USB stick on a blue
// tile with a green "checked" badge. Every size is drawn on its own (with
// supersampling) instead of scaled, so it stays sharp.

type rgb struct{ r, g, b float64 }

var (
	tileTop    = rgb{0x3B, 0x8E, 0xF0}
	tileBottom = rgb{0x17, 0x45, 0x9E}
	white      = rgb{0xFF, 0xFF, 0xFF}
	stickShade = rgb{0xDC, 0xE8, 0xF8}
	metal      = rgb{0xB9, 0xC9, 0xDE}
	hole       = rgb{0x2A, 0x5F, 0xB8}
	green      = rgb{0x22, 0xA0, 0x5B}
)

// roundRect is the signed distance to a rounded rectangle.
func roundRect(x, y, x0, y0, x1, y1, r float64) float64 {
	cx, cy := (x0+x1)/2, (y0+y1)/2
	dx := math.Abs(x-cx) - (x1-x0)/2 + r
	dy := math.Abs(y-cy) - (y1-y0)/2 + r
	return math.Hypot(math.Max(dx, 0), math.Max(dy, 0)) + math.Min(math.Max(dx, dy), 0) - r
}

func circle(x, y, cx, cy, r float64) float64 { return math.Hypot(x-cx, y-cy) - r }

// segment is the distance to a thick line with round ends.
func segment(x, y, ax, ay, bx, by, w float64) float64 {
	t := ((x-ax)*(bx-ax) + (y-ay)*(by-ay)) / ((bx-ax)*(bx-ax) + (by-ay)*(by-ay))
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(x-ax-t*(bx-ax), y-ay-t*(by-ay)) - w/2
}

func mix(a, b rgb, t float64) rgb {
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

// at is the color at a point; ok is false outside the tile. Small icons get
// fewer details and a larger badge.
func at(x, y float64, small bool) (c rgb, ok bool) {
	if roundRect(x, y, 6, 6, 250, 250, 54) > 0 {
		return rgb{}, false
	}
	c = mix(tileTop, tileBottom, (x*0.25+y*0.75)/256)

	// The stick: connector, then body.
	if roundRect(x, y, 78, 34, 148, 110, 8) <= 0 {
		c = metal
		if !small && (roundRect(x, y, 91, 52, 106, 74, 3) <= 0 || roundRect(x, y, 120, 52, 135, 74, 3) <= 0) {
			c = hole
		}
	}
	if roundRect(x, y, 56, 92, 170, 226, 22) <= 0 {
		c = white
		if x > 138 {
			c = mix(white, stickShade, math.Min(1, (x-138)/20))
		}
	}

	// The badge, with a gap in the color of the tile around it.
	bx, by, br, gap, line := 180.0, 182.0, 54.0, 11.0, 18.0
	if small {
		bx, by, br, gap, line = 174, 176, 68, 14, 28
	}
	if circle(x, y, bx, by, br+gap) <= 0 {
		c = mix(tileTop, tileBottom, (x*0.25+y*0.75)/256)
	}
	if circle(x, y, bx, by, br) <= 0 {
		c = green
	}
	k := br / 54
	if segment(x, y, bx-25*k, by+2*k, bx-7*k, by+20*k, line) <= 0 || segment(x, y, bx-7*k, by+20*k, bx+25*k, by-17*k, line) <= 0 {
		c = white
	}
	return c, true
}

// draw renders the icon at size pixels.
func draw(size int) *image.NRGBA {
	const ss = 8 // samples per pixel, in both directions
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for py := 0; py < size; py++ {
		for px := 0; px < size; px++ {
			var sum rgb
			n := 0.0
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					x := (float64(px) + (float64(sx)+0.5)/ss) * 256 / float64(size)
					y := (float64(py) + (float64(sy)+0.5)/ss) * 256 / float64(size)
					if c, ok := at(x, y, size <= 24); ok {
						sum.r, sum.g, sum.b, n = sum.r+c.r, sum.g+c.g, sum.b+c.b, n+1
					}
				}
			}
			if n == 0 {
				continue
			}
			img.SetNRGBA(px, py, color.NRGBA{
				R: uint8(math.Round(sum.r / n)), G: uint8(math.Round(sum.g / n)), B: uint8(math.Round(sum.b / n)),
				A: uint8(math.Round(n / (ss * ss) * 255)),
			})
		}
	}
	return img
}
