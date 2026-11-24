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

// Command genicon draws the program icon and writes it as a Windows icon, a
// PNG and the resource objects (.syso) that "go build" links into the
// Windows programs. Run "make icon" after changing the drawing.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

var sizes = []int{16, 20, 24, 32, 40, 48, 64, 128, 256}

// iconImage is one image in the format icons use: PNG for 256 pixels, a
// bitmap for the smaller ones.
type iconImage struct {
	size int
	data []byte
}

func le(w *bytes.Buffer, values ...any) {
	for _, v := range values {
		_ = binary.Write(w, binary.LittleEndian, v) // a bytes.Buffer never fails
	}
}

func pngBytes(img image.Image) ([]byte, error) {
	var b bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	err := enc.Encode(&b, img)
	return b.Bytes(), err
}

// dib is a 32-bit bitmap as icons hold it: bottom-up, with an (empty) mask.
func dib(img *image.NRGBA) []byte {
	n := img.Rect.Dx()
	maskRow := (n + 31) / 32 * 4
	var b bytes.Buffer
	le(&b, uint32(40), int32(n), int32(2*n), uint16(1), uint16(32), uint32(0), uint32(n*n*4+maskRow*n), int32(0), int32(0), uint32(0), uint32(0))
	for y := n - 1; y >= 0; y-- {
		for x := 0; x < n; x++ {
			c := img.NRGBAAt(x, y)
			b.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	b.Write(make([]byte, maskRow*n))
	return b.Bytes()
}

func images() ([]iconImage, error) {
	var out []iconImage
	for _, s := range sizes {
		img := draw(s)
		data := dib(img)
		if s == 256 {
			var err error
			if data, err = pngBytes(img); err != nil {
				return nil, err
			}
		}
		out = append(out, iconImage{s, data})
	}
	return out, nil
}

// directory writes the icon directory; last writes the end of each entry: a
// file offset (.ico) or a resource number.
func directory(imgs []iconImage, last func(w *bytes.Buffer, i int)) []byte {
	var b bytes.Buffer
	le(&b, uint16(0), uint16(1), uint16(len(imgs)))
	for i, im := range imgs {
		d := uint8(im.size) // 256 is written as 0
		le(&b, d, d, uint8(0), uint8(0), uint16(1), uint16(32), uint32(len(im.data)))
		last(&b, i)
	}
	return b.Bytes()
}

func ico(imgs []iconImage) []byte {
	offset := uint32(6 + 16*len(imgs))
	out := directory(imgs, func(w *bytes.Buffer, i int) {
		le(w, offset)
		offset += uint32(len(imgs[i].data))
	})
	for _, im := range imgs {
		out = append(out, im.data...)
	}
	return out
}

// Windows constants for the resource object.
const (
	machineAMD64   = 0x8664
	machineARM64   = 0xAA64
	relocAMD64     = 3 // IMAGE_REL_AMD64_ADDR32NB
	relocARM64     = 2 // IMAGE_REL_ARM64_ADDR32NB
	rtIcon         = 3
	rtGroupIcon    = 14
	langEnglish    = 0x0409
	subdirectory   = 0x80000000
	groupID        = 1
	dirSize        = 16
	dirEntrySize   = 8
	dataEntrySize  = 16
	fileHeaderSize = 20
	sectionSize    = 40
)

// syso is a COFF object with the icon as resources.
func syso(imgs []iconImage, machine, reloc uint16) []byte {
	type res struct {
		typ, id uint32
		data    []byte
	}
	var all []res
	for i, im := range imgs {
		all = append(all, res{rtIcon, uint32(i + 1), im.data})
	}
	all = append(all, res{rtGroupIcon, groupID, directory(imgs, func(w *bytes.Buffer, i int) { le(w, uint16(i+1)) })})
	icons := len(imgs)

	// Layout: root, the two type directories, a language directory per
	// resource, the data entries, the data.
	typeIcon := uint32(dirSize + 2*dirEntrySize)
	typeGroup := typeIcon + uint32(dirSize+icons*dirEntrySize)
	lang := typeGroup + dirSize + dirEntrySize
	entries := lang + uint32(len(all)*(dirSize+dirEntrySize))
	data := entries + uint32(len(all)*dataEntrySize)

	var b, relocs bytes.Buffer
	dir := func(n int) { le(&b, uint32(0), uint32(0), uint16(0), uint16(0), uint16(0), uint16(n)) }
	dir(2)
	le(&b, uint32(rtIcon), typeIcon|subdirectory, uint32(rtGroupIcon), typeGroup|subdirectory)
	langAt := func(i int) uint32 { return (lang + uint32(i*(dirSize+dirEntrySize))) | subdirectory }
	dir(icons)
	for i := 0; i < icons; i++ {
		le(&b, all[i].id, langAt(i))
	}
	dir(1)
	le(&b, uint32(groupID), langAt(icons))
	for i := range all {
		dir(1)
		le(&b, uint32(langEnglish), entries+uint32(i*dataEntrySize))
	}
	offset := data
	for _, r := range all {
		le(&relocs, uint32(b.Len()), uint32(0), reloc)
		le(&b, offset, uint32(len(r.data)), uint32(0), uint32(0))
		offset += uint32(len(r.data)+7) &^ 7
	}
	for _, r := range all {
		b.Write(r.data)
		for b.Len()%8 != 0 {
			b.WriteByte(0)
		}
	}

	raw := uint32(fileHeaderSize + sectionSize)
	relocAt := raw + uint32(b.Len())
	symAt := relocAt + uint32(relocs.Len())
	var o bytes.Buffer
	le(&o, machine, uint16(1), uint32(0), symAt, uint32(1), uint16(0), uint16(0))
	le(&o, [8]byte{'.', 'r', 's', 'r', 'c'}, uint32(0), uint32(0), uint32(b.Len()), raw, relocAt, uint32(0), uint16(len(all)), uint16(0), uint32(0x40000040))
	o.Write(b.Bytes())
	o.Write(relocs.Bytes())
	le(&o, [8]byte{'.', 'r', 's', 'r', 'c'}, uint32(0), int16(1), uint16(0), uint8(3), uint8(0))
	le(&o, uint32(4)) // empty string table
	return o.Bytes()
}

// files are all generated files, by path from the top of the repository.
func files() (map[string][]byte, error) {
	imgs, err := images()
	if err != nil {
		return nil, err
	}
	p, err := pngBytes(draw(256))
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{
		"packaging/windows/usb-backup.ico": ico(imgs),
		"docs/icon.png":                    p,
	}
	for _, cmd := range []string{"usb-backup", "usb-backup-cli"} {
		out["cmd/"+cmd+"/icon_windows_amd64.syso"] = syso(imgs, machineAMD64, relocAMD64)
		out["cmd/"+cmd+"/icon_windows_arm64.syso"] = syso(imgs, machineARM64, relocARM64)
	}
	return out, nil
}

func main() {
	all, err := files()
	if err == nil {
		for name, data := range all {
			if err = os.WriteFile(filepath.FromSlash(name), data, 0o644); err != nil {
				break
			}
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "genicon:", err)
		os.Exit(1)
	}
}
