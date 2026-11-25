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

// Package gui contains the Windows user interface. It starts
// usb-backup-cli as a child process for all work.
package gui

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/msg"
	"github.com/macsnoeren/backup-usb/internal/week"
)

// Version is the program version, set by main.
var Version = "dev"

const (
	className      = "UsbBackupGuiMain"
	guiMutex       = "UsbBackupGui"
	settingsMutex  = "UsbBackupSettings"
	timerDevice    = 1
	timerOperator  = 2
	timerSettings  = 3
	timerTray      = 4
	deviceSettleMs = 2500

	// Menu commands; windigo numbers its controls down from 0xDFFF.
	cmdSettings = 101
	cmdExit     = 102
	cmdAbout    = 103
	cmdTrayOpen = 104
)

// mainWindow is the status screen.
type mainWindow struct {
	wnd      *ui.Main
	head     *ui.Static
	detail   *ui.Static
	footer   *ui.Static
	progress *ui.ProgressBar
	action   *ui.Button
	gear     *ui.Button
	menu     win.HMENU
	cli      CLI
	p        msg.Printer
	model    *Model
	mu       sync.Mutex
	job      *Job
	auto     bool
	lastView View
	// settingsOnly: this process only shows the settings, as administrator.
	settingsOnly bool
	// settingsOpen: such a process was started and has not ended yet.
	settingsOpen bool

	// The icon next to the clock; see tray_windows.go. Only used in the
	// window's own thread, unless noted otherwise.
	opts     Options
	start    TraySettings
	tray     *trayIcon // nil: off
	schedule week.Schedule
	viewAt   time.Time // when lastView was new
	hinted   bool      // told that the program keeps running
	// Also used elsewhere.
	trayOn   atomic.Bool
	arrivals Arrivals
	diskMu   sync.Mutex // one look at the disks at a time
	pending  bool       // under mu: a disk came during a run
	retrying atomic.Bool
}

// Main shows the window. "--auto" starts a backup at once (USB trigger);
// "--tray" starts hidden, behind the icon (at login); "--settings" only shows the settings, and is how the window opens them as
// administrator. The window itself, and so every backup, keeps running
// without administrator rights.
func Main(args []string) int {
	runtime.LockOSThread()
	o := ParseOptions(args)
	if o.Settings && !singleInstance(settingsMutex) {
		return 0
	}
	if !o.Settings && !singleInstance(guiMutex) {
		secondStart(o)
		return 0
	}
	cli, err := DefaultCLI()
	lang := msg.Default
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), startTimeout(o))
		err = cli.CheckVersion(ctx, Version)
		if err == nil {
			lang = cli.Language(ctx)
		}
		cancel()
	}
	p := msg.Printer{Lang: lang}
	if err != nil {
		_, _ = win.HWND(0).MessageBox(p.T("gui.versionError", err.Error()), p.T("gui.title"), co.MB_ICONERROR)
		return 1
	}
	tray, hidden := TraySettings{}, false
	if !o.Settings {
		// Unreadable settings (changed without the operator): no icon, and
		// the window shows what is wrong.
		ctx, cancel := context.WithTimeout(context.Background(), startTimeout(o))
		tray, err = cli.TraySettings(ctx)
		cancel()
		hidden = o.Tray && err == nil
		if hidden && !tray.Icon {
			return 0 // switched off since it was set to start at login
		}
	}
	m := newMainWindow(cli, p, o, tray, hidden)
	return m.wnd.RunAsMain()
}

// startTimeout is how long the CLI may take at the start: longer when a user
// just logged in and everything starts at once.
func startTimeout(o Options) time.Duration {
	if o.Tray {
		return 2 * time.Minute
	}
	return 20 * time.Second
}

// secondStart passes a start on to the window that is there already.
func secondStart(o Options) {
	h, ok := win.FindWindow(win.ClassNameStr(className), "")
	if !ok {
		return
	}
	id, msgErr := win.RegisterWindowMessage(checkMessage)
	act := SecondStart(o)
	if act == SecondCheck && msgErr == nil {
		_ = win.AllowSetForegroundWindow(asfwAny) // without the icon it comes to the front
		if h.PostMessage(id, checkDisk, 0) == nil {
			return
		}
		act = SecondShow // refused: the window runs as administrator
	}
	if act == SecondShow {
		h.ShowWindow(co.SW_RESTORE)
		h.SetForegroundWindow()
		if msgErr == nil {
			_ = h.PostMessage(id, checkOpened, 0)
		}
	}
}

func newMainWindow(cli CLI, p msg.Printer, o Options, tray TraySettings, hidden bool) *mainWindow {
	settingsOnly := o.Settings
	opts := ui.OptsMain().Title(p.T("gui.title")).Size(ui.Dpi(560, 300)).Center(true)
	var menu win.HMENU
	if settingsOnly {
		opts.ClassName(className + "Settings").Style(co.WS_CAPTION | co.WS_SYSMENU).CmdShow(co.SW_HIDE)
	} else {
		menu = newMainMenu(p.T)
		opts.ClassName(className).Menu(menu)
		if hidden {
			// windigo's own style without WS_VISIBLE, which would show the
			// window before it is hidden.
			opts.CmdShow(co.SW_HIDE).Style(co.WS_CAPTION | co.WS_SYSMENU | co.WS_CLIPCHILDREN | co.WS_BORDER | co.WS_MINIMIZEBOX)
		}
	}
	wnd := ui.NewMain(opts)
	m := &mainWindow{wnd: wnd, menu: menu, cli: cli, p: p, auto: o.Auto, settingsOnly: settingsOnly, opts: o, start: tray, model: NewModel(p.Lang, time.Now)}
	m.gear = ui.NewButton(wnd, ui.OptsButton().Text(p.T("gui.btn.settings")).Position(ui.Dpi(510, 8)).Width(ui.DpiX(36)))
	m.head = ui.NewStatic(wnd, ui.OptsStatic().Text(p.T("gui.checking")).Position(ui.Dpi(20, 40)).Size(ui.Dpi(520, 50)).CtrlStyle(co.SS_CENTER))
	m.detail = ui.NewStatic(wnd, ui.OptsStatic().Text("").Position(ui.Dpi(20, 95)).Size(ui.Dpi(520, 40)).CtrlStyle(co.SS_CENTER))
	m.progress = ui.NewProgressBar(wnd, ui.OptsProgressBar().Position(ui.Dpi(60, 140)).Size(ui.Dpi(440, 18)).Range(0, 100))
	m.action = ui.NewButton(wnd, ui.OptsButton().Text("").Position(ui.Dpi(190, 170)).Width(ui.DpiX(180)))
	m.footer = ui.NewStatic(wnd, ui.OptsStatic().Text("").Position(ui.Dpi(20, 210)).Size(ui.Dpi(520, 60)))
	m.events()
	return m
}

// settingsEvents shows the settings and ends with them.
func (m *mainWindow) settingsEvents() {
	m.wnd.On().WmCreate(func(_ ui.WmCreate) int {
		setWindowIcon(uintptr(m.wnd.Hwnd()))
		_ = m.wnd.Hwnd().SetTimer(timerSettings, 10) // after the message loop started
		return 0
	})
	m.wnd.On().WmTimer(timerSettings, func() {
		_ = m.wnd.Hwnd().KillTimer(timerSettings)
		onGear(m)
		_ = m.wnd.Hwnd().DestroyWindow()
	})
}

// openElevated opens the settings as administrator, in a process of their
// own, and waits for it in the background.
func (m *mainWindow) openElevated() {
	exe, err := os.Executable()
	var wait func()
	if err == nil {
		wait, err = startElevated(uintptr(m.wnd.Hwnd()), exe, settingsArg)
	}
	if errors.Is(err, errDeclined) {
		return
	}
	if err != nil {
		_, _ = m.wnd.Hwnd().MessageBox(err.Error(), m.p.T("gui.title"), co.MB_ICONERROR)
		return
	}
	m.settingsOpen = true
	m.render(m.lastView)
	go func() {
		wait()
		m.wnd.UiThread(func() {
			m.settingsOpen = false
			m.render(m.lastView)
		})
		m.reloadTray()
		m.refreshStatus()
	}()
}

func (m *mainWindow) events() {
	if m.settingsOnly {
		m.settingsEvents()
		return
	}
	m.wnd.On().WmCreate(func(_ ui.WmCreate) int {
		setWindowIcon(uintptr(m.wnd.Hwnd()))
		m.viewAt = time.Now()
		tray := m.setTray(m.start)
		m.render(m.model.View())
		go func() {
			if tray {
				m.baseline() // first: refresh finds a disk that comes now
			}
			m.refresh(m.auto, m.opts.Tray)
		}()
		return 0
	})
	// A disk was plugged in: check again after the drive letter settled.
	m.wnd.On().WmDeviceChange(func(p ui.WmDeviceChange) co.BROADCAST_QUERY {
		switch p.Event() {
		case co.DBT_DEVICEARRIVAL:
			_ = m.wnd.Hwnd().SetTimer(timerDevice, deviceSettleMs)
		case co.DBT_DEVICEREMOVECOMPLETE:
			if m.tray != nil {
				m.arrivals.Removed(volumeLetters(p))
			}
		}
		return co.BROADCAST_QUERY_OK
	})
	m.wnd.On().WmTimer(timerDevice, func() {
		_ = m.wnd.Hwnd().KillTimer(timerDevice)
		m.deviceSettled()
	})
	m.trayEvents()
	m.action.On().BnClicked(func() { m.onAction() })
	settings := func() {
		onGear(m)
		go m.reloadTray()
	}
	m.gear.On().BnClicked(settings)
	m.wnd.On().WmCommandAccelMenu(cmdSettings, settings)
	m.wnd.On().WmCommandAccelMenu(cmdExit, func() { m.exit() })
	m.wnd.On().WmCommandAccelMenu(cmdAbout, func() { m.showAbout(m.wnd) })
	m.wnd.On().WmClose(func() {
		if m.hide() {
			return
		}
		if m.busy() {
			return // never close during a backup; use Cancel
		}
		m.quit()
	})
}

// newMainMenu makes the menu bar; 0 (no menu) if Windows refuses.
func newMainMenu(t func(string, ...any) string) win.HMENU {
	bar, err := win.CreateMenu()
	if err != nil {
		return 0
	}
	file, err1 := win.CreatePopupMenu()
	help, err2 := win.CreatePopupMenu()
	if err1 != nil || err2 != nil {
		return 0
	}
	appendMenu(file, cmdSettings, t("gui.menu.settings"))
	appendMenuSeparator(file)
	appendMenu(file, cmdExit, t("gui.menu.exit"))
	appendMenu(help, cmdAbout, t("gui.menu.about"))
	appendSubMenu(bar, file, t("gui.menu.file"))
	appendSubMenu(bar, help, t("gui.menu.help"))
	return bar
}

// showAbout shows version, settings folder, key state and license. It needs
// no password.
func (m *mainWindow) showAbout(parent ui.Parent) {
	a := AboutInfo{Version: Version, Go: runtime.Version(), Platform: runtime.GOOS + "/" + runtime.GOARCH, Folder: config.Dir()}
	a.Keys, a.KeysErr = m.cli.KeysState(context.Background())
	_, _ = parentHwnd(parent).MessageBox(AboutText(m.p, a), m.p.T("gui.tab.about"), co.MB_ICONINFORMATION)
}

func (m *mainWindow) busy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.job != nil
}

// refresh reads the status (footer, disk that is due) and, if start is
// set, starts a backup; otherwise a dry run shows which disk is expected.
// passive: the user did not start the program, but logged in or clicked the
// icon.
func (m *mainWindow) refresh(start, passive bool) {
	ctx := context.Background()
	m.refreshStatus()
	if start {
		m.startRun(runOptions{args: []string{"--wait", "30"}})
		return
	}
	// Show "insert X" or "wrong disk" without writing anything.
	res := m.cli.Call(ctx, nil, "run", "--dry-run")
	m.wnd.UiThread(func() {
		if m.tray != nil && m.busy() {
			return // a backup started meanwhile; its messages count
		}
		m.viewAt = time.Now()
		switch {
		case res.Event == "error":
			v := m.model.Apply(res)
			m.render(v)
			m.attend(&Attendant{}, res, v)
		case !StartWhenReady(m.schedule, passive):
			m.render(View{State: StateNoDisk, Headline: m.p.T("gui.waiting"), Footer: m.model.View().Footer, Progress: -1})
		default:
			m.render(View{State: StateChecking, Headline: m.p.T("gui.checking"), Footer: m.model.View().Footer, Progress: -1})
			go m.refresh(true, passive)
		}
	})
}

// refreshStatus reads the status for the footer and the disk that is due.
func (m *mainWindow) refreshStatus() {
	ev := m.cli.Call(context.Background(), nil, "status")
	var st app.StatusReport
	if json.Unmarshal(ev.Report, &st) == nil {
		m.wnd.UiThread(func() { m.model.SetStatus(&st); m.render(m.model.View()) })
	}
}

// runOptions say why a run starts.
type runOptions struct {
	args []string
	// inserted: because a registered disk was plugged in.
	inserted bool
	// fresh are the new disks it is for; more: others came at the same time.
	fresh []string
	more  bool
}

// startRun starts a backup; false when one is running.
func (m *mainWindow) startRun(o runOptions) bool {
	m.mu.Lock()
	if m.job != nil {
		m.mu.Unlock()
		return false
	}
	att := &Attendant{Inserted: o.inserted} // used in the window's thread
	j, err := m.cli.Start(context.Background(), nil, append([]string{"run"}, o.args...)...)
	if err != nil {
		m.mu.Unlock()
		m.wnd.UiThread(func() {
			v := View{State: StateFailed, Color: Red, Headline: m.p.T("gui.failed"), Detail: err.Error(), Progress: -1, Button: ButtonRetry}
			m.render(v)
			m.attend(att, Event{Event: "error"}, v)
		})
		return true
	}
	m.job = j
	m.pending = o.more
	m.mu.Unlock()
	go func() {
		var last Event
		for e := range j.Events {
			e := e
			last = e
			m.arrivals.Seen(e.Disk)
			m.wnd.UiThread(func() {
				v := m.model.Apply(e)
				m.viewAt = time.Now()
				m.render(v)
				m.attend(att, e, v)
			})
		}
		m.mu.Lock()
		m.job = nil
		pending := m.pending
		m.pending = false
		m.mu.Unlock()
		// Footer with the new numbers; an ejected disk is not listed any
		// more, then the numbers of the backup report stay.
		ev := m.cli.Call(context.Background(), nil, "status")
		var st app.StatusReport
		if json.Unmarshal(ev.Report, &st) == nil && len(st.Disks) > 0 {
			m.wnd.UiThread(func() {
				m.model.SetStatus(&st)
				v := m.lastView
				v.Footer = m.model.View().Footer
				m.render(v)
			})
		}
		m.runEnded(o, last, pending)
	}()
	return true
}

func (m *mainWindow) onAction() {
	switch m.lastView.Button {
	case ButtonCancel:
		m.mu.Lock()
		if m.job != nil {
			m.job.Cancel()
		}
		m.mu.Unlock()
	case ButtonRetry:
		go m.refresh(true, false)
	case ButtonDetails:
		_, _ = m.wnd.Hwnd().MessageBox(m.lastView.Details, m.p.T("gui.btn.details"), co.MB_ICONINFORMATION)
	case ButtonEject:
		go func() {
			ev := m.cli.Call(context.Background(), nil, "eject")
			m.wnd.UiThread(func() {
				if ev.Event == "error" {
					_, _ = m.wnd.Hwnd().MessageBox(ev.Message, m.p.T("gui.title"), co.MB_ICONWARNING)
					return
				}
				var r map[string]string
				_ = json.Unmarshal(ev.Report, &r)
				v := m.lastView
				v.Headline, v.Button, v.Detail = m.p.T("gui.ejected", r["ejected"]), ButtonNone, ""
				m.render(v)
			})
		}()
	}
}

// render draws a view.
func (m *mainWindow) render(v View) {
	m.lastView = v
	_ = m.head.Hwnd().SetWindowText(v.Headline)
	_ = m.detail.Hwnd().SetWindowText(v.Detail)
	if v.Footer != "" {
		_ = m.footer.Hwnd().SetWindowText(v.Footer)
	}
	if v.Progress < 0 {
		m.progress.Hwnd().ShowWindow(co.SW_HIDE)
	} else {
		m.progress.Hwnd().ShowWindow(co.SW_SHOW)
		m.progress.SetPos(v.Progress)
		switch v.Color {
		case Red:
			m.progress.SetState(co.PBST_ERROR)
		case Orange:
			m.progress.SetState(co.PBST_PAUSED)
		default:
			m.progress.SetState(co.PBST_NORMAL)
		}
	}
	label := map[Button]string{ButtonCancel: "gui.btn.cancel", ButtonEject: "gui.btn.eject", ButtonDetails: "gui.btn.details", ButtonRetry: "gui.btn.retry"}[v.Button]
	if label == "" {
		m.action.Hwnd().ShowWindow(co.SW_HIDE)
	} else {
		m.action.SetText(m.p.T(label))
		m.action.Hwnd().ShowWindow(co.SW_SHOW)
	}
	icon := map[Color]string{Green: "✔ ", Orange: "⚠ ", Red: "✖ "}[v.Color]
	_ = m.wnd.Hwnd().SetWindowText(strings.TrimSpace(icon + m.p.T("gui.title")))
	// The settings and closing are not available while a backup runs.
	if m.tray != nil {
		m.tray.setTip(TrayTip(m.p, v))
	}
	idle := v.State != StateBusy && v.State != StateVerifying && !m.settingsOpen
	m.gear.Hwnd().EnableWindow(idle)
	if m.menu != 0 {
		_ = m.menu.EnableMenuItemByCmd(idle, cmdSettings, cmdExit)
	}
}
