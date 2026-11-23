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
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/msg"
)

// Version is the program version, set by main.
var Version = "dev"

const (
	className      = "UsbBackupGuiMain"
	timerDevice    = 1
	timerOperator  = 2
	deviceSettleMs = 2500

	// Menu commands; windigo numbers its controls down from 0xDFFF.
	cmdSettings = 101
	cmdExit     = 102
	cmdAbout    = 103
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
}

// Main shows the window. "--auto" starts a backup at once (USB trigger).
func Main(args []string) int {
	runtime.LockOSThread()
	if !singleInstance() {
		if h, ok := win.FindWindow(win.ClassNameStr(className), ""); ok {
			h.ShowWindow(co.SW_RESTORE)
			h.SetForegroundWindow()
		}
		return 0
	}
	cli, err := DefaultCLI()
	lang := msg.Default
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
	auto := false
	for _, a := range args[1:] {
		auto = auto || a == "--auto"
	}
	m := newMainWindow(cli, p, auto)
	return m.wnd.RunAsMain()
}

func newMainWindow(cli CLI, p msg.Printer, auto bool) *mainWindow {
	menu := newMainMenu(p.T)
	wnd := ui.NewMain(ui.OptsMain().
		Menu(menu).
		ClassName(className).
		Title(p.T("gui.title")).
		Size(ui.Dpi(560, 300)).
		Center(true))
	m := &mainWindow{wnd: wnd, menu: menu, cli: cli, p: p, auto: auto, model: NewModel(p.Lang, time.Now)}
	m.gear = ui.NewButton(wnd, ui.OptsButton().Text(p.T("gui.btn.settings")).Position(ui.Dpi(510, 8)).Width(ui.DpiX(36)))
	m.head = ui.NewStatic(wnd, ui.OptsStatic().Text(p.T("gui.checking")).Position(ui.Dpi(20, 40)).Size(ui.Dpi(520, 50)).CtrlStyle(co.SS_CENTER))
	m.detail = ui.NewStatic(wnd, ui.OptsStatic().Text("").Position(ui.Dpi(20, 95)).Size(ui.Dpi(520, 40)).CtrlStyle(co.SS_CENTER))
	m.progress = ui.NewProgressBar(wnd, ui.OptsProgressBar().Position(ui.Dpi(60, 140)).Size(ui.Dpi(440, 18)).Range(0, 100))
	m.action = ui.NewButton(wnd, ui.OptsButton().Text("").Position(ui.Dpi(190, 170)).Width(ui.DpiX(180)))
	m.footer = ui.NewStatic(wnd, ui.OptsStatic().Text("").Position(ui.Dpi(20, 210)).Size(ui.Dpi(520, 60)))
	m.events()
	return m
}

func (m *mainWindow) events() {
	m.wnd.On().WmCreate(func(_ ui.WmCreate) int {
		m.render(m.model.View())
		go m.refresh(m.auto)
		return 0
	})
	// A disk was plugged in: check again after the drive letter settled.
	m.wnd.On().WmDeviceChange(func(p ui.WmDeviceChange) co.BROADCAST_QUERY {
		if p.Event() == co.DBT_DEVICEARRIVAL {
			_ = m.wnd.Hwnd().SetTimer(timerDevice, deviceSettleMs)
		}
		return co.BROADCAST_QUERY_OK
	})
	m.wnd.On().WmTimer(timerDevice, func() {
		_ = m.wnd.Hwnd().KillTimer(timerDevice)
		if !m.busy() {
			go m.refresh(true)
		}
	})
	m.action.On().BnClicked(func() { m.onAction() })
	m.gear.On().BnClicked(func() { onGear(m) })
	m.wnd.On().WmCommandAccelMenu(cmdSettings, func() { onGear(m) })
	m.wnd.On().WmCommandAccelMenu(cmdExit, func() { _, _ = m.wnd.Hwnd().SendMessage(co.WM_CLOSE, 0, 0) })
	m.wnd.On().WmCommandAccelMenu(cmdAbout, func() { m.showAbout(m.wnd) })
	m.wnd.On().WmClose(func() {
		if m.busy() {
			return // never close during a backup; use Cancel
		}
		_ = m.wnd.Hwnd().DestroyWindow()
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
func (m *mainWindow) refresh(start bool) {
	ctx := context.Background()
	ev := m.cli.Call(ctx, nil, "status")
	var st app.StatusReport
	if json.Unmarshal(ev.Report, &st) == nil {
		m.wnd.UiThread(func() { m.model.SetStatus(&st); m.render(m.model.View()) })
	}
	if start {
		m.startRun("--wait", "30")
		return
	}
	// Show "insert X" or "wrong disk" without writing anything.
	res := m.cli.Call(ctx, nil, "run", "--dry-run")
	m.wnd.UiThread(func() {
		if res.Event == "error" {
			m.render(m.model.Apply(res))
		} else {
			m.render(View{State: StateChecking, Headline: m.p.T("gui.checking"), Footer: m.model.View().Footer, Progress: -1})
			go m.refresh(true)
		}
	})
}

func (m *mainWindow) startRun(extra ...string) {
	m.mu.Lock()
	if m.job != nil {
		m.mu.Unlock()
		return
	}
	j, err := m.cli.Start(context.Background(), nil, append([]string{"run"}, extra...)...)
	if err != nil {
		m.mu.Unlock()
		m.wnd.UiThread(func() {
			m.render(View{State: StateFailed, Color: Red, Headline: m.p.T("gui.failed"), Detail: err.Error(), Progress: -1, Button: ButtonRetry})
		})
		return
	}
	m.job = j
	m.mu.Unlock()
	go func() {
		for e := range j.Events {
			e := e
			m.wnd.UiThread(func() { m.render(m.model.Apply(e)) })
		}
		m.mu.Lock()
		m.job = nil
		m.mu.Unlock()
		// Footer with the new last-backup time.
		ev := m.cli.Call(context.Background(), nil, "status")
		var st app.StatusReport
		if json.Unmarshal(ev.Report, &st) == nil {
			m.wnd.UiThread(func() { m.model.SetStatus(&st); m.render(m.lastView) })
		}
	}()
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
		go m.refresh(true)
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
	idle := v.State != StateBusy && v.State != StateVerifying
	m.gear.Hwnd().EnableWindow(idle)
	if m.menu != 0 {
		_ = m.menu.EnableMenuItemByCmd(idle, cmdSettings, cmdExit)
	}
}
