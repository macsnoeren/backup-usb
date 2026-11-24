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

package gui

import (
	"syscall"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
)

// newModal creates a modal window.
func newModal(parent ui.Parent, title string, w, h int) *ui.Modal {
	return ui.NewModal(parent, ui.OptsModal().Title(title).Size(ui.Dpi(w, h)))
}

func closeModal(m *ui.Modal) { _, _ = m.Hwnd().SendMessage(co.WM_CLOSE, 0, 0) }

func label(p ui.Parent, text string, x, y, w, h int) *ui.Static {
	return ui.NewStatic(p, ui.OptsStatic().Text(text).Position(ui.Dpi(x, y)).Size(ui.Dpi(w, h)))
}

// okButton is the default button: Enter in a field presses it.
func okButton(p ui.Parent, text string, x, y, w int) *ui.Button {
	return ui.NewButton(p, ui.OptsButton().Text(text).Position(ui.Dpi(x, y)).Width(ui.DpiX(w)).
		CtrlId(uint16(co.ID_OK)).CtrlStyle(co.BS_DEFPUSHBUTTON))
}

// cancelButton is pressed by Escape.
func cancelButton(p ui.Parent, text string, x, y, w int) *ui.Button {
	return ui.NewButton(p, ui.OptsButton().Text(text).Position(ui.Dpi(x, y)).Width(ui.DpiX(w)).CtrlId(uint16(co.ID_CANCEL)))
}

// isEmpty reports whether a field has no text, without reading it.
func isEmpty(e *ui.Edit) bool {
	n, _, _ := procGetWindowTextLength.Call(uintptr(e.Hwnd()))
	return n == 0
}

func button(p ui.Parent, text string, x, y, w int) *ui.Button {
	return ui.NewButton(p, ui.OptsButton().Text(text).Position(ui.Dpi(x, y)).Width(ui.DpiX(w)))
}

func edit(p ui.Parent, text string, x, y, w int, password bool) *ui.Edit {
	o := ui.OptsEdit().Text(text).Position(ui.Dpi(x, y)).Width(ui.DpiX(w))
	if password {
		// ES_PASSWORD shows dots and blocks copy and cut.
		o = o.CtrlStyle(co.ES_AUTOHSCROLL | co.ES_PASSWORD)
	}
	return ui.NewEdit(p, o)
}

func listView(p ui.Parent, x, y, w, h int, cols ...string) *ui.ListView {
	o := ui.OptsListView().Position(ui.Dpi(x, y)).Size(ui.Dpi(w, h)).
		CtrlStyle(co.LVS_REPORT | co.LVS_SHOWSELALWAYS).CtrlExStyle(co.LVS_EX_FULLROWSELECT)
	for _, c := range cols {
		o = o.Column(c, ui.DpiX(w/len(cols)))
	}
	return ui.NewListView(p, o)
}

// combo makes a drop-down list with selected chosen. windigo creates it with
// height 0, and without version 6 of the common controls Windows takes that
// as the height of the list: nothing to choose from. showDropDowns fixes it.
func combo(p ui.Parent, x, y, w, selected int, items ...string) *ui.ComboBox {
	return ui.NewComboBox(p, ui.OptsComboBox().Position(ui.Dpi(x, y)).Width(ui.DpiX(w)).CtrlStyle(co.CBS_DROPDOWNLIST).Texts(items...).Select(selected))
}

// showDropDowns gives drop-down lists room to open; Windows shrinks the
// list to the number of items. Call it from WM_CREATE on.
func showDropDowns(boxes ...*ui.ComboBox) {
	for _, b := range boxes {
		rc, err := b.Hwnd().GetWindowRect()
		if err != nil {
			continue
		}
		_ = b.Hwnd().SetWindowPos(win.HWND(0), win.POINT{},
			win.SIZE{Cx: rc.Right - rc.Left, Cy: int32(ui.DpiY(220))}, //nolint:gosec // a small constant
			co.SWP_NOMOVE|co.SWP_NOZORDER|co.SWP_NOACTIVATE)
	}
}

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	procGetWindowText       = user32.NewProc("GetWindowTextW")
)

// takeSecret reads a password field straight into a UTF-16 buffer, turns
// it into UTF-8 bytes, wipes the buffer and clears the field. No Go string
// copy of the password is made; the control's own memory is outside our
// reach.
func takeSecret(e *ui.Edit) []byte {
	h := uintptr(e.Hwnd())
	n, _, _ := procGetWindowTextLength.Call(h)
	if n == 0 || n > 4096 {
		e.SetText("")
		return nil
	}
	buf := make([]uint16, n+1)
	procGetWindowText.Call(h, uintptr(unsafe.Pointer(&buf[0])), n+1) //nolint:errcheck,gosec // Win32 call
	out := make([]byte, 0, n*3)
	for _, r := range utf16.Decode(buf[:n]) {
		out = utf8.AppendRune(out, r)
	}
	clear(buf)
	e.SetText("")
	return out
}

// askText asks for one line of text.
func askText(parent ui.Parent, title, prompt, initial string, p func(string, ...any) string) (string, bool) {
	m := newModal(parent, title, 380, 130)
	// windigo gives the focus to the control that was made first.
	e := edit(m, initial, 10, 35, 360, false)
	label(m, prompt, 10, 10, 360, 20)
	ok := okButton(m, p("gui.btn.ok"), 200, 75, 80)
	cancel := cancelButton(m, p("gui.btn.close"), 290, 75, 80)
	var result string
	var done bool
	ok.On().BnClicked(func() { result, done = e.Text(), true; closeModal(m) })
	cancel.On().BnClicked(func() { closeModal(m) })
	m.ShowModal()
	return result, done
}
