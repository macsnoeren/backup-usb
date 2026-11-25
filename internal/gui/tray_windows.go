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
	"context"
	"errors"
	"strings"
	"time"
	"unsafe"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
	"github.com/rodrigocfd/windigo/x/cosh"
	"github.com/rodrigocfd/windigo/x/winsh"
)

const (
	trayID      = 1
	wmTray      = co.WM_APP + 1
	trayRetryMs = 5000

	// checkMessage from a second start: wParam 0, a disk may be there; 1,
	// the window was opened.
	checkMessage = "UsbBackupCheckDisks"
	checkDisk    = 0
	checkOpened  = 1

	// busyRetry is when a disk gets another chance after another backup,
	// restore or check was running.
	busyRetry = time.Minute
)

// trayIcon is the icon next to the clock.
type trayIcon struct {
	hwnd  win.HWND
	icon  win.HICON
	tip   string
	shown bool
}

func (t *trayIcon) data(flags cosh.NIF) *winsh.NOTIFYICONDATA {
	d := &winsh.NOTIFYICONDATA{HWnd: t.hwnd, UID: trayID, UFlags: flags, UCallbackMessage: wmTray, HIcon: t.icon}
	d.SetCbSize()
	d.SetSzTip(t.tip)
	return d
}

// add shows the icon; it fails when there is no taskbar (yet). Adding can
// time out although the icon is there, so then it is changed instead.
func (t *trayIcon) add() bool {
	const flags = cosh.NIF_MESSAGE | cosh.NIF_ICON | cosh.NIF_TIP
	t.shown = winsh.Shell_NotifyIcon(cosh.NIM_ADD, t.data(flags)) == nil ||
		winsh.Shell_NotifyIcon(cosh.NIM_MODIFY, t.data(flags)) == nil
	return t.shown
}

func (t *trayIcon) remove() {
	if t.shown {
		_ = winsh.Shell_NotifyIcon(cosh.NIM_DELETE, t.data(0))
		t.shown = false
	}
}

func (t *trayIcon) setTip(tip string) {
	if tip == t.tip {
		return
	}
	t.tip = tip
	if t.shown {
		_ = winsh.Shell_NotifyIcon(cosh.NIM_MODIFY, t.data(cosh.NIF_TIP))
	}
}

// notify shows a message at the icon.
func (t *trayIcon) notify(title, text string) {
	if !t.shown {
		return
	}
	d := t.data(cosh.NIF_INFO)
	d.SetSzInfoTitle(title)
	d.SetSzInfo(text)
	d.DwInfoFlags = cosh.NIIF_INFO | cosh.NIIF_RESPECT_QUIET_TIME
	_ = winsh.Shell_NotifyIcon(cosh.NIM_MODIFY, d)
}

// programIcon loads the small program icon (resource 1).
func programIcon() win.HICON {
	const imageIcon, smCxSmIcon, smCySmIcon = 1, 49, 50
	module, _, _ := procGetModuleHandle.Call(0)
	w, _, _ := procGetSystemMetrics.Call(smCxSmIcon)
	h, _, _ := procGetSystemMetrics.Call(smCySmIcon)
	icon, _, _ := procLoadImage.Call(module, 1, imageIcon, w, h, 0)
	return win.HICON(icon)
}

var procIsWindowEnabled = user32.NewProc("IsWindowEnabled")

// trayEvents handles the messages of the icon and of a second start.
func (m *mainWindow) trayEvents() {
	m.wnd.On().Wm(wmTray, func(p ui.Wm) uintptr {
		switch co.WM(uint32(p.LParam) & 0xFFFF) { //nolint:gosec // the mouse message is in the low word
		case co.WM_LBUTTONUP:
			m.open()
		case co.WM_RBUTTONUP:
			m.trayMenu()
		}
		return 0
	})
	// Explorer started again: the icon is gone.
	if id, err := win.RegisterWindowMessage("TaskbarCreated"); err == nil {
		m.wnd.On().Wm(id, func(_ ui.Wm) uintptr {
			if m.tray != nil {
				m.addIcon()
			}
			return 0
		})
	}
	if id, err := win.RegisterWindowMessage(checkMessage); err == nil {
		m.wnd.On().Wm(id, func(p ui.Wm) uintptr {
			switch {
			case p.WParam == checkOpened:
				m.renew()
			case m.tray == nil:
				m.show()
			default:
				_ = m.wnd.Hwnd().SetTimer(timerDevice, deviceSettleMs)
			}
			return 0
		})
	}
	m.wnd.On().WmTimer(timerTray, func() {
		if m.tray == nil || m.tray.add() {
			_ = m.wnd.Hwnd().KillTimer(timerTray)
		}
	})
}

// addIcon shows the icon, again and again until there is a taskbar (at
// login the window can be first).
func (m *mainWindow) addIcon() {
	if !m.tray.add() {
		_ = m.wnd.Hwnd().SetTimer(timerTray, trayRetryMs)
	}
}

// setTray switches the icon on or off; it reports whether it was switched
// on.
func (m *mainWindow) setTray(s TraySettings) bool {
	m.schedule = s.Schedule
	m.trayOn.Store(s.Icon)
	switch {
	case s.Icon && m.tray == nil:
		m.tray = &trayIcon{hwnd: m.wnd.Hwnd(), icon: programIcon(), tip: TrayTip(m.p, m.lastView)}
		m.addIcon()
		return true
	case !s.Icon && m.tray != nil:
		m.tray.remove()
		m.tray = nil
		if !m.wnd.Hwnd().IsWindowVisible() {
			m.show() // nothing else leads to it any more
		}
	}
	return false
}

// reloadTray takes over a changed setting.
func (m *mainWindow) reloadTray() {
	s, err := m.cli.TraySettings(context.Background())
	if err != nil {
		return
	}
	on := false
	m.wnd.UiThread(func() { on = m.setTray(s) })
	if on {
		m.baseline()
	}
}

// visible reports whether the user can see the window (not hidden, not
// minimised).
func (m *mainWindow) visible() bool {
	h := m.wnd.Hwnd()
	return h.IsWindowVisible() && !h.IsIconic()
}

func (m *mainWindow) show() {
	h := m.wnd.Hwnd()
	h.ShowWindow(co.SW_RESTORE)
	h.SetForegroundWindow()
}

// bringForward shows the window on top without the user asking for it. It
// does not take the focus: a key meant for another program must not press
// Cancel.
func (m *mainWindow) bringForward() {
	m.wnd.Hwnd().ShowWindow(co.SW_SHOWNOACTIVATE)
	h, flags := m.wnd.Hwnd(), co.SWP_NOMOVE|co.SWP_NOSIZE|co.SWP_NOACTIVATE
	topmost, noTopmost := win.HWND(^uintptr(0)), win.HWND(^uintptr(1))
	_ = h.SetWindowPos(topmost, win.POINT{}, win.SIZE{}, flags)
	_ = h.SetWindowPos(noTopmost, win.POINT{}, win.SIZE{}, flags)
}

// open shows the window because the user asked for it.
func (m *mainWindow) open() {
	m.show()
	m.renew()
}

// renew replaces an old message; looking is not plugging in.
func (m *mainWindow) renew() {
	if m.tray != nil && !m.busy() && Stale(m.lastView, time.Since(m.viewAt)) {
		m.viewAt = time.Now()
		go m.refresh(false, true)
	}
}

// hide leaves only the icon; false when there is no icon to come back from.
func (m *mainWindow) hide() bool {
	if m.tray == nil || !m.tray.shown {
		return false
	}
	m.wnd.Hwnd().ShowWindow(co.SW_HIDE)
	if !m.hinted {
		m.hinted = true
		m.tray.notify(m.p.T("gui.title"), m.p.T("gui.tray.running"))
	}
	return true
}

// canExit: not during a backup, not while the settings are open.
func (m *mainWindow) canExit() bool {
	enabled, _, _ := procIsWindowEnabled.Call(uintptr(m.wnd.Hwnd())) // not behind a dialog
	return enabled != 0 && !m.settingsOpen && !m.busy()
}

// exit ends the program, also with the icon on.
func (m *mainWindow) exit() {
	if m.tray == nil {
		_, _ = m.wnd.Hwnd().SendMessage(co.WM_CLOSE, 0, 0)
		return
	}
	if m.canExit() {
		m.quit()
	}
}

func (m *mainWindow) quit() {
	if m.tray != nil {
		m.tray.remove()
	}
	_ = m.wnd.Hwnd().DestroyWindow()
}

// trayMenu shows the menu of the icon.
func (m *mainWindow) trayMenu() {
	menu, err := win.CreatePopupMenu()
	if err != nil {
		return
	}
	defer func() { _ = menu.DestroyMenu() }()
	const mfGrayed = 1
	exitFlags := uintptr(0)
	if !m.canExit() {
		exitFlags = mfGrayed
	}
	appendMenu(menu, cmdTrayOpen, m.p.T("gui.tray.open"))
	appendMenuSeparator(menu)
	appendMenuItem(menu, exitFlags, cmdExit, m.p.T("gui.menu.exit"))
	pos, err := win.GetCursorPos()
	if err != nil {
		return
	}
	h := m.wnd.Hwnd()
	h.SetForegroundWindow() // or the menu stays when the user clicks elsewhere
	id, _ := menu.TrackPopupMenu(co.TPM_RETURNCMD|co.TPM_RIGHTBUTTON, int(pos.X), int(pos.Y), h)
	_ = h.PostMessage(co.WM_NULL, 0, 0)
	switch id {
	case cmdTrayOpen:
		m.open()
	case cmdExit:
		m.exit()
	}
}

// attend brings a hidden window forward when the event asks for it.
func (m *mainWindow) attend(a *Attendant, e Event, v View) {
	att := a.Next(e, v, m.visible())
	if m.tray == nil {
		return
	}
	switch att {
	case AttentionShow:
		m.bringForward()
	case AttentionNotify:
		m.tray.notify(m.p.T("gui.title"), strings.TrimSpace(v.Headline+"\n"+v.Detail))
	}
}

// volumeLetters are the drives a device event is about.
func volumeLetters(p ui.WmDeviceChange) string {
	const typeVolume = 2
	type volumeEvent struct{ size, deviceType, reserved, unitMask uint32 }
	v := (*volumeEvent)(p.EventData())
	if v == nil || v.size < uint32(unsafe.Sizeof(*v)) || v.deviceType != typeVolume {
		return ""
	}
	return DriveLetters(v.unitMask)
}

var errDisksChanging = errors.New("disks keep changing")

// updateDisks asks which registered disks are connected and which of them
// are new. One at a time, so an old answer never replaces a newer one.
func (m *mainWindow) updateDisks() ([]string, Connected, error) {
	m.diskMu.Lock()
	defer m.diskMu.Unlock()
	for range 5 {
		gen := m.arrivals.Gen()
		c, err := m.cli.ConnectedDisks(context.Background())
		if err != nil {
			return nil, Connected{}, err
		}
		if fresh, ok := m.arrivals.Update(c.Mounts, gen); ok {
			return fresh, c, nil
		}
	}
	return nil, Connected{}, errDisksChanging
}

// baseline notes the disks that are there already.
func (m *mainWindow) baseline() { _, _, _ = m.updateDisks() }

// afterRun forgets the disks that are gone.
func (m *mainWindow) afterRun() {
	m.diskMu.Lock()
	defer m.diskMu.Unlock()
	if c, err := m.cli.ConnectedDisks(context.Background()); err == nil {
		m.arrivals.Refresh(c.Mounts)
	}
}

// deviceSettled is called when the drive letter of a new disk is there.
func (m *mainWindow) deviceSettled() {
	if m.tray == nil {
		if !m.busy() {
			go m.refresh(true, false)
		}
		return
	}
	switch {
	case m.setPending():
	case m.visible():
		go func() {
			m.baseline()
			m.refresh(true, false)
		}()
	default:
		go m.checkDisks(checkOptions{inserted: true})
	}
}

// setPending remembers during a run that a disk came; false when nothing
// runs.
func (m *mainWindow) setPending() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pending = m.pending || m.job != nil
	return m.job != nil
}

// checkOptions say how checkDisks treats new disks.
type checkOptions struct {
	// inserted: plugged in just now, so the user is told when a disk is not
	// used.
	inserted bool
	// onlyDue: no run and no message for a disk that is not due.
	onlyDue bool
}

// checkDisks starts a backup when a registered disk is there that was not
// there before. Reading a disk can take a while after it was plugged in.
func (m *mainWindow) checkDisks(o checkOptions) {
	for _, wait := range []time.Duration{0, 2 * time.Second, 4 * time.Second, 8 * time.Second} {
		time.Sleep(wait)
		if !m.trayOn.Load() {
			return
		}
		fresh, c, err := m.updateDisks()
		if err != nil && !errors.Is(err, errDisksChanging) {
			m.startRun(runOptions{inserted: o.inserted}) // the run tells what is wrong
			return
		}
		if len(fresh) == 0 {
			continue
		}
		run := runOptions{inserted: o.inserted, fresh: fresh}
		target, rest := Target(fresh, c)
		if target != "" {
			run.fresh, run.args = []string{target}, []string{"--target", c.Mounts[target]}
		} else if e, other := WrongDisk(fresh, c); other || o.onlyDue {
			if other && o.inserted && !o.onlyDue {
				m.wnd.UiThread(func() { m.tell(e) })
			}
			return
		}
		if len(rest) > 0 {
			m.arrivals.Forget(rest...)
			run.more = true
		}
		if m.startRun(run) {
			return
		}
		// Another run just started; these disks are next.
		m.arrivals.Forget(fresh...)
		if m.setPending() {
			return
		}
	}
}

// tell shows a message that no run made, unless one just started.
func (m *mainWindow) tell(e Event) {
	if m.busy() {
		return
	}
	v := m.model.Apply(e)
	m.viewAt = time.Now()
	m.render(v)
	m.attend(&Attendant{Inserted: true}, e, v)
}

// retryLater gives disks another chance after another backup, restore or
// check; without messages about disks that are not used.
func (m *mainWindow) retryLater(fresh []string) {
	m.arrivals.Forget(fresh...)
	if m.retrying.Swap(true) {
		return // one timer is enough
	}
	time.AfterFunc(busyRetry, func() {
		m.retrying.Store(false)
		if !m.setPending() {
			m.checkDisks(checkOptions{})
		}
	})
}

// runEnded is called after a run, with its last event.
func (m *mainWindow) runEnded(o runOptions, last Event, pending bool) {
	if !m.trayOn.Load() {
		return
	}
	follow := FollowNone
	m.wnd.UiThread(func() { follow = FollowUp(last, m.lastView, pending, len(o.fresh) > 0) })
	if follow == FollowLater {
		m.retryLater(o.fresh)
		return
	}
	m.afterRun()
	switch {
	case follow != FollowNone:
		m.checkDisks(checkOptions{inserted: true, onlyDue: follow == FollowDue})
	case pending:
		m.baseline() // or the next stick would make them new
	}
}
