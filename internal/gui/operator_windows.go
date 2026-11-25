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
	"encoding/json"
	"fmt"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"

	"github.com/macsnoeren/backup-usb/internal/app"
	"github.com/macsnoeren/backup-usb/internal/config"
	"github.com/macsnoeren/backup-usb/internal/password"
	"github.com/macsnoeren/backup-usb/internal/secmem"
	"github.com/macsnoeren/backup-usb/internal/week"
)

// operator is an open operator session.
type operator struct {
	main    *mainWindow
	session *Session
}

func (o *operator) t(key string, args ...any) string { return o.main.p.T(key, args...) }

// call runs a CLI command with the vault password; Touch keeps the session.
func (o *operator) call(args ...string) Event {
	pw := o.session.Password()
	if pw == nil {
		return Event{Event: "error", Code: 13, Message: o.t("gui.loginWrong")}
	}
	defer clear(pw)
	return o.main.cli.Call(context.Background(), [][]byte{pw}, args...)
}

// plain runs a CLI command without a password.
func (o *operator) plain(args ...string) Event {
	o.session.Touch()
	return o.main.cli.Call(context.Background(), nil, args...)
}

func (o *operator) report(parent ui.Parent, ev Event, okText string) bool {
	h := parentHwnd(parent)
	if ev.Event == "error" {
		_, _ = h.MessageBox(ev.Message, o.t("gui.title"), co.MB_ICONERROR)
		return false
	}
	if okText != "" {
		_, _ = h.MessageBox(okText, o.t("gui.title"), co.MB_ICONINFORMATION)
	}
	return true
}

// watchTimeout closes the modal when the session expired. The boxes are the
// drop-down lists of the modal; a window has only one WM_CREATE handler.
func (o *operator) watchTimeout(m *ui.Modal, boxes ...*ui.ComboBox) {
	m.On().WmCreate(func(_ ui.WmCreate) int {
		showDropDowns(boxes...)
		_ = m.Hwnd().SetTimer(timerOperator, 5000)
		return 0
	})
	m.On().WmTimer(timerOperator, func() {
		if o.session.Expired() {
			_ = m.Hwnd().KillTimer(timerOperator)
			closeModal(m)
		}
	})
}

// onGear opens the settings: after the vault password, or, when there are
// no keys yet, after the first-time setup.
func onGear(mw *mainWindow) {
	p, h := mw.p.T, mw.wnd.Hwnd()
	st, err := mw.cli.KeysState(context.Background())
	if err != nil {
		_, _ = h.MessageBox(err.Error(), p("gui.title"), co.MB_ICONERROR)
		return
	}
	if NeedsAdmin(st) && !mw.settingsOnly {
		if ActionFor(st) == GearNeedAdmin {
			if id, _ := h.MessageBox(p("gui.askAdminSetup"), p("gui.title"), co.MB_OKCANCEL|co.MB_ICONINFORMATION); id == co.ID_OK {
				mw.openElevated()
			}
			return
		}
		switch id, _ := h.MessageBox(p("gui.askAdmin"), p("gui.title"), co.MB_YESNOCANCEL|co.MB_ICONINFORMATION); id {
		case co.ID_YES:
			mw.openElevated()
			return
		case co.ID_NO:
		default:
			return
		}
	}
	switch ActionFor(st) {
	case GearSetup:
		firstSetup(mw)
	case GearNeedAdmin:
		_, _ = h.MessageBox(p("gui.needAdmin"), p("gui.title"), co.MB_ICONINFORMATION)
	case GearNoVault:
		_, _ = h.MessageBox(p("gui.noVault", st.Vault), p("gui.title"), co.MB_ICONWARNING)
	case GearLoginIncomplete:
		_, _ = h.MessageBox(p("gui.keysIncomplete", st.Vault), p("gui.title"), co.MB_ICONWARNING)
		showOperatorLogin(mw)
	default:
		showOperatorLogin(mw)
	}
}

// firstSetup makes the keys on a fresh installation and opens the settings
// with the new password. The CLI refuses when there are keys already.
func firstSetup(mw *mainWindow) {
	o := &operator{main: mw}
	h := mw.wnd.Hwnd()
	if id, _ := h.MessageBox(o.t("gui.firstSetup"), o.t("gui.title"), co.MB_OKCANCEL|co.MB_ICONINFORMATION); id != co.ID_OK {
		return
	}
	pw := o.askNewPassword(mw.wnd)
	if pw == nil {
		return
	}
	file := ""
	for {
		file = fileDialog(uintptr(h), true, o.t("gui.breakglassTitle"), "usb-backup-breakglass.txt")
		if file == "" {
			pw.Wipe()
			return
		}
		if IsNewFile(file) {
			break
		}
		_, _ = h.MessageBox(o.t("err.fileExists", file), o.t("gui.title"), co.MB_ICONWARNING)
	}
	ev := mw.cli.Call(context.Background(), [][]byte{pw.Bytes(), pw.Bytes()}, "keys", "init", "--breakglass-out", file)
	if !o.report(mw.wnd, ev, "") {
		pw.Wipe()
		return
	}
	o.breakglassScreen(mw.wnd, file)
	o.session = NewSession(pw, mw.cli.OperatorTimeout(context.Background()), time.Now) // takes pw over
	o.settings()
	o.session.End()
}

// showOperatorLogin asks the vault password and opens the settings.
func showOperatorLogin(mw *mainWindow) {
	p := mw.p.T
	m := newModal(mw.wnd, p("gui.title"), 380, 130)
	// windigo gives the focus to the control that was made first, also when
	// the window comes back after a message: type the password at once.
	pwEdit := edit(m, "", 10, 35, 360, true)
	label(m, p("gui.login"), 10, 10, 360, 20)
	ok := okButton(m, p("gui.btn.ok"), 200, 75, 80)
	cancel := cancelButton(m, p("gui.btn.close"), 290, 75, 80)
	var sess *Session
	ok.On().BnClicked(func() {
		raw := takeSecret(pwEdit)
		pw := secmem.From(raw)
		ev := mw.cli.Call(context.Background(), [][]byte{pw.Bytes()}, "keys", "status")
		if ev.Event != "result" {
			pw.Wipe()
			text := ev.Message
			if ev.Key == "err.vaultPassword" || text == "" {
				text = p("gui.loginWrong")
			}
			_, _ = m.Hwnd().MessageBox(text, p("gui.title"), co.MB_ICONWARNING)
			return
		}
		sess = NewSession(pw, mw.cli.OperatorTimeout(context.Background()), time.Now)
		closeModal(m)
	})
	cancel.On().BnClicked(func() { closeModal(m) })
	m.ShowModal()
	if sess == nil {
		return
	}
	o := &operator{main: mw, session: sess}
	o.settings()
	o.session.End() // a password change replaced the session
}

// settings is the operator screen: one button per section.
func (o *operator) settings() {
	m := newModal(o.main.wnd, o.t("gui.settings"), 320, 300)
	o.watchTimeout(m)
	sections := []struct {
		key string
		fn  func(ui.Parent)
	}{
		{"gui.tab.general", o.general},
		{"gui.tab.disks", o.disks},
		{"gui.tab.keys", o.keys},
		{"gui.tab.restore", o.restore},
		{"gui.tab.log", o.logAndLanguage},
		{"gui.tab.about", o.about},
	}
	for i, s := range sections {
		s := s
		b := button(m, o.t(s.key), 20, 15+i*40, 280)
		b.On().BnClicked(func() {
			if o.session.Expired() {
				closeModal(m)
				return
			}
			s.fn(m)
		})
	}
	m.ShowModal()
}

// about shows the version and the license notice (GPL-3.0-or-later).
func (o *operator) about(parent ui.Parent) {
	o.main.showAbout(parent)
}

// --- General and sources (T11.6) ---

func (o *operator) loadConfig() *config.Config {
	o.session.Touch()
	cfg, err := o.main.cli.Config(context.Background())
	if err != nil {
		return config.Defaults()
	}
	return cfg
}

func (o *operator) general(parent ui.Parent) {
	m := newModal(parent, o.t("gui.tab.general"), 640, 567)
	srcList := listView(m, 10, 10, 620, 130, o.t("gui.tag"), o.t("gui.path"), o.t("gui.exclude"))
	add := button(m, o.t("gui.addSource"), 10, 148, 140)
	remove := button(m, o.t("gui.removeSource"), 160, 148, 100)
	setList := listView(m, 10, 185, 620, 210, o.t("gui.setting"), o.t("gui.value"))
	value := edit(m, "", 10, 405, 500, false)
	save := button(m, o.t("gui.save"), 520, 403, 110)
	label(m, o.t("gui.schedule"), 10, 448, 130, 20)
	var often []string
	for _, s := range week.Schedules() {
		often = append(often, o.t("gui.schedule."+string(s)))
	}
	schedule := combo(m, 145, 445, 340, 0, often...)
	saveSchedule := button(m, o.t("gui.save"), 520, 443, 110)
	usb := ui.NewCheckBox(m, ui.OptsCheckBox().Text(o.t("gui.usbTrigger")).Position(ui.Dpi(10, 482)).Size(ui.Dpi(620, 20)))
	tray := ui.NewCheckBox(m, ui.OptsCheckBox().Text(o.t("gui.trayIcon")).Position(ui.Dpi(10, 508)).Size(ui.Dpi(620, 20)))
	login := ui.NewCheckBox(m, ui.OptsCheckBox().Text(o.t("gui.startAtLogin")).Position(ui.Dpi(10, 534)).Size(ui.Dpi(620, 20)))
	fillStart := func() {
		st := o.main.cli.starts(context.Background())
		usb.SetCheck(st.USB)
		login.SetCheck(st.Login)
		login.Hwnd().EnableWindow(tray.IsChecked() || st.Login)
	}
	fill := func() {
		cfg := o.loadConfig()
		tray.SetCheck(cfg.TrayIcon)
		srcList.DeleteAllItems()
		for _, s := range cfg.Sources {
			srcList.AddItem(s.Tag, s.Path, strings.Join(s.Exclude, ", "))
		}
		setList.DeleteAllItems()
		for _, k := range config.Keys() {
			if k == "sources" {
				continue
			}
			v, _ := cfg.Get(k)
			setList.AddItem(k, v)
		}
		for i, s := range week.Schedules() {
			if s == cfg.Schedule {
				schedule.SelectIndex(i)
			}
		}
	}
	o.watchTimeout(m)
	m.On().WmCreate(func(_ ui.WmCreate) int {
		showDropDowns(schedule)
		fill()
		fillStart()
		_ = m.Hwnd().SetTimer(timerOperator, 5000)
		return 0
	})
	saveSchedule.On().BnClicked(func() {
		i := schedule.SelectedIndex()
		if i < 0 {
			return
		}
		if o.report(m, o.call("config", "set", "schedule", string(week.Schedules()[i])), o.t("gui.saved")) {
			fill()
		}
	})
	usb.On().BnClicked(func() {
		// Takes effect at once; the box shows what really is the case.
		args := []string{"uninstall", "--trigger", "usb"}
		if usb.IsChecked() {
			args = []string{"install"}
		}
		o.report(m, o.plain(args...), "")
		fillStart()
	})
	trayChanged := func(on bool) {
		if !on && login.IsChecked() {
			o.report(m, o.plain("uninstall", "--trigger", "login"), "")
		}
		if on && usb.IsChecked() {
			// Without a time limit for the window that stays. The task becomes
			// one of this user, who can be another one (the administrator).
			if id, _ := m.Hwnd().MessageBox(o.t("gui.taskAgain", userName()), o.t("gui.title"), co.MB_YESNO|co.MB_ICONQUESTION); id == co.ID_YES {
				o.report(m, o.plain("install"), "")
			}
		}
	}
	tray.On().BnClicked(func() {
		on := tray.IsChecked()
		if o.report(m, o.call("config", "set", "trayIcon", strconv.FormatBool(on)), "") {
			trayChanged(on)
		}
		fill()
		fillStart()
	})
	login.On().BnClicked(func() {
		cmd := "uninstall"
		if login.IsChecked() {
			cmd = "install"
		}
		o.report(m, o.plain(cmd, "--trigger", "login"), "")
		fillStart()
	})
	setList.On().LvnItemChanged(func(_ *winNMLISTVIEW) {
		if items := setList.SelectedItems(); len(items) == 1 {
			value.SetText(strings.Trim(items[0].Text(1), `"`))
		}
	})
	add.On().BnClicked(func() {
		dir := pickFolder(uintptr(m.Hwnd()), o.t("gui.addSource"))
		if dir == "" {
			return
		}
		tag, ok := askText(m, o.t("gui.addSource"), o.t("gui.tag"), filepath.Base(dir), o.t)
		if !ok {
			return
		}
		if o.report(m, o.call("config", "add-source", "--tag", tag, "--path", dir), "") {
			fill()
		}
	})
	remove.On().BnClicked(func() {
		items := srcList.SelectedItems()
		if len(items) != 1 {
			return
		}
		if o.report(m, o.call("config", "remove-source", items[0].Text(0)), "") {
			fill()
		}
	})
	save.On().BnClicked(func() {
		items := setList.SelectedItems()
		if len(items) != 1 {
			return
		}
		key := items[0].Text(0)
		before := tray.IsChecked()
		if o.report(m, o.call("config", "set", key, value.Text()), o.t("gui.saved")) {
			fill()
			if on := tray.IsChecked(); on != before {
				trayChanged(on)
				fillStart()
			}
		}
	})
	m.ShowModal()
}

// --- Disks (T11.7) ---

func (o *operator) disks(parent ui.Parent) {
	m := newModal(parent, o.t("gui.tab.disks"), 640, 380)
	o.watchTimeout(m)
	regList := listView(m, 10, 10, 620, 120, o.t("gui.name"), o.t("gui.lastBackup"), o.t("gui.connected"))
	forget := button(m, o.t("gui.forget"), 10, 138, 120)
	label(m, o.t("gui.disksInTurn"), 140, 135, 490, 36)
	conList := listView(m, 10, 175, 620, 120, o.t("gui.path"), "label", "state")
	label(m, o.t("gui.name"), 10, 305, 60, 20)
	name := edit(m, "", 70, 302, 140, false)
	register := button(m, o.t("gui.register"), 220, 300, 120)
	var rep app.DisksReport
	fill := func() {
		ev := o.plain("disks")
		_ = json.Unmarshal(ev.Report, &rep)
		regList.DeleteAllItems()
		for _, r := range rep.Registered {
			c := "-"
			if r.Connected {
				c = r.Mount
			}
			regList.AddItem(r.Name, DiskLastBackup(o.main.p, r), c)
		}
		name.SetText(NextDiskName(rep.Registered))
		conList.DeleteAllItems()
		for _, v := range rep.Connected {
			conList.AddItem(v.Mount, v.Label, v.State+" "+v.Name)
		}
	}
	m.On().WmCreate(func(_ ui.WmCreate) int {
		fill()
		_ = m.Hwnd().SetTimer(timerOperator, 5000)
		return 0
	})
	register.On().BnClicked(func() {
		items := conList.SelectedItems()
		if len(items) != 1 {
			return
		}
		if o.report(m, o.call("enroll", items[0].Text(0), "--name", name.Text(), "--force"), o.t("gui.saved")) {
			fill()
		}
	})
	forget.On().BnClicked(func() {
		items := regList.SelectedItems()
		if len(items) != 1 {
			return
		}
		if o.report(m, o.call("forget", items[0].Text(0)), "") {
			fill()
		}
	})
	m.ShowModal()
}

// --- Keys (T11.8) ---

// askNewPassword asks a new vault password twice (at least 12 characters).
func (o *operator) askNewPassword(parent ui.Parent) *secmem.Buffer {
	m := newModal(parent, o.t("gui.tab.keys"), 400, 190)
	p1 := edit(m, "", 10, 32, 380, true) // first: gets the focus
	label(m, o.t("gui.newPassword"), 10, 10, 380, 20)
	label(m, o.t("gui.repeatPassword"), 10, 65, 380, 20)
	p2 := edit(m, "", 10, 87, 380, true)
	ok := okButton(m, o.t("gui.btn.ok"), 220, 130, 80)
	cancel := cancelButton(m, o.t("gui.btn.close"), 310, 130, 80)
	var out *secmem.Buffer
	ok.On().BnClicked(func() {
		if !isEmpty(p1) && isEmpty(p2) {
			_, _ = p2.Hwnd().SetFocus() // Enter in the first field
			return
		}
		a, b := secmem.From(takeSecret(p1)), secmem.From(takeSecret(p2))
		defer b.Wipe()
		if !a.Equal(b) || utf8.RuneCount(a.Bytes()) < password.MinNewLen {
			a.Wipe()
			_, _ = m.Hwnd().MessageBox(o.t("gui.passwordRules"), o.t("gui.title"), co.MB_ICONWARNING)
			return
		}
		out = a
		closeModal(m)
	})
	cancel.On().BnClicked(func() { closeModal(m) })
	m.ShowModal()
	return out
}

// breakglassScreen shows where the key was saved and only continues after
// the operator confirms it is stored; closing is refused until then.
func (o *operator) breakglassScreen(parent ui.Parent, file string) {
	m := newModal(parent, o.t("gui.breakglassTitle"), 520, 260)
	label(m, o.t("gui.breakglassSaved", file), 10, 10, 500, 140)
	print := button(m, o.t("gui.print"), 10, 160, 120)
	confirm := ui.NewCheckBox(m, ui.OptsCheckBox().Text(o.t("gui.confirmStored")).Position(ui.Dpi(10, 195)).Size(ui.Dpi(360, 20)))
	cont := button(m, o.t("gui.continue"), 400, 192, 110)
	// Controls only exist from WM_CREATE on.
	m.On().WmCreate(func(_ ui.WmCreate) int { cont.Hwnd().EnableWindow(false); return 0 })
	confirmed := false
	print.On().BnClicked(func() { _ = shellExecute("print", file) })
	confirm.On().BnClicked(func() { cont.Hwnd().EnableWindow(confirm.IsChecked()) })
	cont.On().BnClicked(func() {
		if confirm.IsChecked() {
			confirmed = true
			closeModal(m)
		}
	})
	m.On().WmClose(func() {
		if confirmed {
			// This handler replaces windigo's own, which enables the parent
			// again; without it the window behind stays frozen.
			if owner, err := m.Hwnd().GetWindow(co.GW_OWNER); err == nil {
				owner.EnableWindow(true)
			}
			_ = m.Hwnd().DestroyWindow()
		}
	})
	m.ShowModal()
}

func (o *operator) keys(parent ui.Parent) {
	m := newModal(parent, o.t("gui.tab.keys"), 320, 180)
	o.watchTimeout(m)
	// Making the keys happens before there is a password: see firstSetup.
	passwd := button(m, o.t("gui.changePassword"), 20, 15, 280)
	rotate := button(m, o.t("gui.rotateKey"), 20, 55, 280)
	update := button(m, o.t("gui.updateDisk"), 20, 95, 280)
	passwd.On().BnClicked(func() {
		pw := o.askNewPassword(m)
		if pw == nil {
			return
		}
		defer pw.Wipe()
		old := o.session.Password()
		defer clear(old)
		ev := o.main.cli.Call(context.Background(), [][]byte{old, pw.Bytes(), pw.Bytes()}, "keys", "passwd")
		if o.report(m, ev, o.t("gui.saved")) {
			o.session.End()
			o.session = NewSession(secmem.From(append([]byte(nil), pw.Bytes()...)), o.session.timeout, time.Now)
		}
	})
	rotate.On().BnClicked(func() {
		if id, _ := m.Hwnd().MessageBox(o.t("gui.rotateKey")+"?", o.t("gui.title"), co.MB_YESNO|co.MB_ICONQUESTION); id != co.ID_YES {
			return
		}
		ev := o.call("keys", "rotate")
		o.report(m, ev, ev.Text())
	})
	update.On().BnClicked(func() {
		ev := o.call("keys", "rewrap", "--keep-old", "--json")
		if !o.report(m, ev, "") {
			return
		}
		var r app.RewrapReport
		_ = json.Unmarshal([]byte(ev.Text()), &r)
		for _, id := range r.OldKeysFree {
			if ans, _ := m.Hwnd().MessageBox(o.t("keys.oldFree", id), o.t("gui.title"), co.MB_YESNO|co.MB_ICONQUESTION); ans == co.ID_YES {
				o.report(m, o.call("keys", "rewrap", "--remove-old"), o.t("keys.oldRemoved", id))
			}
		}
		_, _ = m.Hwnd().MessageBox(o.t("keys.rewrapped", r.Disk, r.Rewrapped, r.Current), o.t("gui.title"), co.MB_ICONINFORMATION)
	})
	m.ShowModal()
}

// --- Browse and restore (T11.10) ---

func (o *operator) restore(parent ui.Parent) {
	m := newModal(parent, o.t("gui.tab.restore"), 860, 480)
	o.watchTimeout(m)
	periods := listView(m, 10, 10, 260, 410, o.t("gui.periods"), o.t("gui.tag"), "disk")
	search := edit(m, "", 280, 10, 380, false)
	find := button(m, o.t("gui.search"), 670, 8, 90)
	up := button(m, o.t("gui.up"), 770, 8, 80)
	files := listView(m, 280, 40, 570, 380, o.t("gui.files"), o.t("gui.size"), o.t("gui.modified"))
	toOrig := button(m, o.t("gui.restoreOriginal"), 280, 430, 230)
	toDir := button(m, o.t("gui.restoreTo"), 520, 430, 110)
	toDesktop := button(m, o.t("gui.restoreDesktop"), 640, 430, 210)
	bg := button(m, o.t("gui.testBreakglass"), 10, 430, 260)
	var cur struct{ week, tag, path string }
	var entries []app.ListEntry
	showFiles := func(args ...string) {
		if cur.tag == "" {
			return
		}
		ev := o.call(append([]string{"list", "--week", cur.week, "--tag", cur.tag}, args...)...)
		if !o.report(m, ev, "") {
			return
		}
		var res app.ListResult
		_ = json.Unmarshal(ev.Report, &res)
		entries = res.Entries
		files.DeleteAllItems()
		for _, e := range entries {
			name := e.Path
			if e.Dir {
				name += "/"
			}
			mod := ""
			if !e.Modified.IsZero() {
				mod = e.Modified.Local().Format("2006-01-02 15:04")
			}
			files.AddItem(name, app.HumanBytes(e.Size), mod)
		}
	}
	m.On().WmCreate(func(_ ui.WmCreate) int {
		_ = m.Hwnd().SetTimer(timerOperator, 5000)
		ev := o.plain("list")
		var res app.ListResult
		_ = json.Unmarshal(ev.Report, &res)
		sort.Slice(res.Periods, func(i, j int) bool { return res.Periods[i].Period > res.Periods[j].Period })
		for _, p := range res.Periods {
			for _, t := range p.Tags {
				periods.AddItem(p.Period, t, p.Disk)
			}
		}
		return 0
	})
	periods.On().LvnItemActivate(func(_ *winNMITEMACTIVATE) {
		items := periods.SelectedItems()
		if len(items) != 1 {
			return
		}
		cur.week, cur.tag, cur.path = items[0].Text(0), items[0].Text(1), ""
		showFiles()
	})
	files.On().LvnItemActivate(func(_ *winNMITEMACTIVATE) {
		items := files.SelectedItems()
		if len(items) == 1 && strings.HasSuffix(items[0].Text(0), "/") {
			cur.path = strings.TrimSuffix(items[0].Text(0), "/")
			showFiles("--path", cur.path)
		}
	})
	up.On().BnClicked(func() {
		if i := strings.LastIndex(cur.path, "/"); i >= 0 {
			cur.path = cur.path[:i]
		} else {
			cur.path = ""
		}
		showFiles("--path", cur.path)
	})
	find.On().BnClicked(func() { showFiles("--search", search.Text()) })
	selected := func() []string {
		var out []string
		for _, it := range files.SelectedItems() {
			out = append(out, strings.TrimSuffix(it.Text(0), "/"))
		}
		return out
	}
	// doRestore asks where to only when something is selected; a question
	// that is cancelled restores nothing.
	doRestore := func(where func() (dest []string, ok bool)) {
		sel := selected()
		if len(sel) == 0 {
			_, _ = m.Hwnd().MessageBox(o.t("gui.selectFiles"), o.t("gui.title"), co.MB_ICONINFORMATION)
			return
		}
		dest, ok := where()
		if !ok {
			return
		}
		args := []string{"restore", "--week", cur.week, "--tag", cur.tag}
		for _, f := range sel {
			args = append(args, "--file", f)
		}
		ev := o.call(append(args, dest...)...)
		if !o.report(m, ev, "") {
			return
		}
		var r app.RestoreReport
		_ = json.Unmarshal(ev.Report, &r)
		if ans, _ := m.Hwnd().MessageBox(o.t("gui.restored", r.Files, r.Destination)+"\n\n"+o.t("gui.openFolder"), o.t("gui.title"), co.MB_YESNO|co.MB_ICONINFORMATION); ans == co.ID_YES {
			_ = shellExecute("open", r.Destination)
		}
	}
	toOrig.On().BnClicked(func() { doRestore(func() ([]string, bool) { return []string{"--original"}, true }) })
	toDir.On().BnClicked(func() {
		doRestore(func() ([]string, bool) { return RestoreTo(pickFolder(uintptr(m.Hwnd()), o.t("gui.restoreTo"))) })
	})
	toDesktop.On().BnClicked(func() {
		doRestore(func() ([]string, bool) { return RestoreTo(desktopRestored()) })
	})
	bg.On().BnClicked(func() {
		file := fileDialog(uintptr(m.Hwnd()), false, o.t("gui.testBreakglass"), "")
		if file == "" {
			return
		}
		ev := o.plain("verify", "--identity", file)
		o.report(m, ev, o.t("gui.breakglassOK"))
	})
	m.ShowModal()
}

// --- Log and language (T11.11) ---

func (o *operator) logAndLanguage(parent ui.Parent) {
	m := newModal(parent, o.t("gui.tab.log"), 360, 160)
	open := button(m, o.t("gui.openLog"), 20, 15, 320)
	label(m, o.t("gui.language"), 20, 65, 100, 20)
	current := 0
	if o.main.p.Lang == "nl" {
		current = 1
	}
	lang := combo(m, 120, 62, 120, current, "English", "Nederlands")
	save := button(m, o.t("gui.save"), 250, 60, 90)
	o.watchTimeout(m, lang)
	open.On().BnClicked(func() {
		cfg := o.loadConfig()
		_ = shellExecute("open", filepath.Join(cfg.LogDir(config.Dir()), config.LogFileName))
	})
	save.On().BnClicked(func() {
		l := "en"
		if lang.SelectedIndex() == 1 {
			l = "nl"
		}
		o.report(m, o.call("config", "set", "language", l), fmt.Sprintf("%s (%s)", o.t("gui.saved"), l))
	})
	m.ShowModal()
}

// userName is the Windows user of this process, for messages.
func userName() string {
	u, err := user.Current()
	if err != nil {
		return "?"
	}
	return u.Username
}
