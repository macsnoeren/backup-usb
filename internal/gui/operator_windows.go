package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

// watchTimeout closes the modal when the session expired.
func (o *operator) watchTimeout(m *ui.Modal) {
	m.On().WmCreate(func(_ ui.WmCreate) int {
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

// showOperatorLogin asks the vault password and opens the settings.
func showOperatorLogin(mw *mainWindow) {
	p := mw.p.T
	m := newModal(mw.wnd, p("gui.title"), 380, 130)
	label(m, p("gui.login"), 10, 10, 360, 20)
	pwEdit := edit(m, "", 10, 35, 360, true)
	ok := button(m, p("gui.btn.ok"), 200, 75, 80)
	cancel := button(m, p("gui.btn.close"), 290, 75, 80)
	var sess *Session
	ok.On().BnClicked(func() {
		raw := takeSecret(pwEdit)
		pw := secmem.From(raw)
		ev := mw.cli.Call(context.Background(), [][]byte{pw.Bytes()}, "keys", "status")
		if ev.Code != 0 {
			pw.Wipe()
			_, _ = m.Hwnd().MessageBox(p("gui.loginWrong"), p("gui.title"), co.MB_ICONWARNING)
			return
		}
		timeout := 5 * time.Minute
		if ev2 := mw.cli.Call(context.Background(), nil, "config", "show", "operatorTimeoutMinutes"); ev2.Event == "result" {
			if n, err := strconv.Atoi(strings.TrimSpace(ev2.Output)); err == nil && n > 0 {
				timeout = time.Duration(n) * time.Minute
			}
		}
		sess = NewSession(pw, timeout, time.Now)
		closeModal(m)
	})
	cancel.On().BnClicked(func() { closeModal(m) })
	m.ShowModal()
	if sess == nil {
		return
	}
	o := &operator{main: mw, session: sess}
	o.settings()
	sess.End()
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
		{"gui.tab.schedule", o.schedule},
		{"gui.tab.restore", o.restore},
		{"gui.tab.log", o.logAndLanguage},
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

// --- General and sources (T11.6) ---

func (o *operator) loadConfig() *config.Config {
	ev := o.plain("config", "show")
	cfg, err := config.Parse([]byte(ev.Output))
	if err != nil {
		return config.Defaults()
	}
	return cfg
}

func (o *operator) general(parent ui.Parent) {
	m := newModal(parent, o.t("gui.tab.general"), 640, 470)
	o.watchTimeout(m)
	srcList := listView(m, 10, 10, 620, 130, o.t("gui.tag"), o.t("gui.path"), o.t("gui.exclude"))
	add := button(m, o.t("gui.addSource"), 10, 148, 140)
	remove := button(m, o.t("gui.removeSource"), 160, 148, 100)
	setList := listView(m, 10, 185, 620, 210, o.t("gui.setting"), o.t("gui.value"))
	value := edit(m, "", 10, 405, 500, false)
	save := button(m, o.t("gui.save"), 520, 403, 110)
	fill := func() {
		cfg := o.loadConfig()
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
	}
	m.On().WmInitDialog(func(_ ui.WmInitDialog) bool { fill(); return true })
	m.On().WmCreate(func(_ ui.WmCreate) int { fill(); _ = m.Hwnd().SetTimer(timerOperator, 5000); return 0 })
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
		if o.report(m, o.call("config", "set", items[0].Text(0), value.Text()), o.t("gui.saved")) {
			fill()
		}
	})
	m.ShowModal()
}

// --- Disks (T11.7) ---

func (o *operator) disks(parent ui.Parent) {
	m := newModal(parent, o.t("gui.tab.disks"), 640, 380)
	o.watchTimeout(m)
	regList := listView(m, 10, 10, 620, 120, o.t("gui.name"), "weeks", o.t("gui.connected"))
	forget := button(m, o.t("gui.forget"), 10, 138, 120)
	conList := listView(m, 10, 175, 620, 120, o.t("gui.path"), "label", "state")
	label(m, o.t("gui.name"), 10, 305, 60, 20)
	name := edit(m, "BACKUP1", 70, 302, 140, false)
	weeks := combo(m, 220, 302, 150, o.t("gui.weeksOdd"), o.t("gui.weeksEven"))
	register := button(m, o.t("gui.register"), 380, 300, 120)
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
			regList.AddItem(r.Name, r.Weeks, c)
		}
		conList.DeleteAllItems()
		for _, v := range rep.Connected {
			conList.AddItem(v.Mount, v.Label, v.State+" "+v.Name)
		}
	}
	m.On().WmCreate(func(_ ui.WmCreate) int { fill(); _ = m.Hwnd().SetTimer(timerOperator, 5000); return 0 })
	register.On().BnClicked(func() {
		items := conList.SelectedItems()
		if len(items) != 1 {
			return
		}
		w := "odd"
		if weeks.SelectedIndex() == 1 {
			w = "even"
		}
		if o.report(m, o.call("enroll", items[0].Text(0), "--name", name.Text(), "--weeks", w, "--force"), o.t("gui.saved")) {
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
	label(m, o.t("gui.newPassword"), 10, 10, 380, 20)
	p1 := edit(m, "", 10, 32, 380, true)
	label(m, o.t("gui.repeatPassword"), 10, 65, 380, 20)
	p2 := edit(m, "", 10, 87, 380, true)
	ok := button(m, o.t("gui.btn.ok"), 220, 130, 80)
	cancel := button(m, o.t("gui.btn.close"), 310, 130, 80)
	var out *secmem.Buffer
	ok.On().BnClicked(func() {
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
	cont.Hwnd().EnableWindow(false)
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
			_ = m.Hwnd().DestroyWindow()
		}
	})
	m.ShowModal()
}

func (o *operator) keys(parent ui.Parent) {
	m := newModal(parent, o.t("gui.tab.keys"), 320, 220)
	o.watchTimeout(m)
	setup := button(m, o.t("gui.setupKeys"), 20, 15, 280)
	passwd := button(m, o.t("gui.changePassword"), 20, 55, 280)
	rotate := button(m, o.t("gui.rotateKey"), 20, 95, 280)
	update := button(m, o.t("gui.updateDisk"), 20, 135, 280)
	setup.On().BnClicked(func() {
		pw := o.askNewPassword(m)
		if pw == nil {
			return
		}
		defer pw.Wipe()
		file := fileDialog(uintptr(m.Hwnd()), true, o.t("gui.breakglassTitle"), "usb-backup-breakglass.txt")
		if file == "" {
			return
		}
		// A save dialog may pick an existing file; keys init only writes
		// new files, so it must not exist.
		_ = os.Remove(file)
		ev := o.main.cli.Call(context.Background(), [][]byte{pw.Bytes(), pw.Bytes()}, "keys", "init", "--breakglass-out", file)
		if o.report(m, ev, "") {
			o.breakglassScreen(m, file)
		}
	})
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
		var r app.RotateResult
		_ = json.Unmarshal([]byte(ev.Output), &r)
		o.report(m, ev, strings.TrimSpace(ev.Output))
	})
	update.On().BnClicked(func() {
		ev := o.call("keys", "rewrap", "--keep-old", "--json")
		if !o.report(m, ev, "") {
			return
		}
		var r app.RewrapReport
		_ = json.Unmarshal([]byte(ev.Output), &r)
		for _, id := range r.OldKeysFree {
			if ans, _ := m.Hwnd().MessageBox(o.t("keys.oldFree", id), o.t("gui.title"), co.MB_YESNO|co.MB_ICONQUESTION); ans == co.ID_YES {
				o.report(m, o.call("keys", "rewrap", "--remove-old"), o.t("keys.oldRemoved", id))
			}
		}
		_, _ = m.Hwnd().MessageBox(o.t("keys.rewrapped", r.Disk, r.Rewrapped, r.Current), o.t("gui.title"), co.MB_ICONINFORMATION)
	})
	m.ShowModal()
}

// --- Schedule (T11.9) ---

func (o *operator) schedule(parent ui.Parent) {
	m := newModal(parent, o.t("gui.tab.schedule"), 460, 220)
	o.watchTimeout(m)
	usb := ui.NewCheckBox(m, ui.OptsCheckBox().Text(o.t("gui.usbTrigger")).Position(ui.Dpi(10, 15)).Size(ui.Dpi(440, 20)))
	usb.SetCheck(true)
	label(m, o.t("gui.scheduled"), 10, 55, 140, 20)
	kind := combo(m, 150, 52, 140, o.t("gui.none"), o.t("gui.daily"), o.t("gui.weekly"))
	label(m, o.t("gui.time"), 10, 90, 140, 20)
	at := edit(m, "02:00", 150, 87, 80, false)
	label(m, o.t("gui.day"), 10, 125, 140, 20)
	day := combo(m, 150, 122, 140, "mon", "tue", "wed", "thu", "fri", "sat", "sun")
	save := button(m, o.t("gui.save"), 330, 165, 110)
	save.On().BnClicked(func() {
		var evs []Event
		if usb.IsChecked() {
			evs = append(evs, o.plain("install", "--trigger", "usb"))
		} else {
			evs = append(evs, o.plain("uninstall", "--trigger", "usb"))
		}
		evs = append(evs, o.plain("uninstall", "--trigger", "daily"), o.plain("uninstall", "--trigger", "weekly"))
		switch kind.SelectedIndex() {
		case 1:
			evs = append(evs, o.plain("install", "--trigger", "daily", "--at", at.Text()))
		case 2:
			evs = append(evs, o.plain("install", "--trigger", "weekly", "--day", day.CurrentText(), "--at", at.Text()))
		}
		for i, ev := range evs {
			if i == 1 || i == 2 {
				continue // removing a task that did not exist is fine
			}
			if !o.report(m, ev, "") {
				return
			}
		}
		_, _ = m.Hwnd().MessageBox(o.t("gui.saved"), o.t("gui.title"), co.MB_ICONINFORMATION)
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
	toDir := button(m, o.t("gui.restoreTo"), 520, 430, 150)
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
	doRestore := func(dest []string) {
		sel := selected()
		if len(sel) == 0 {
			_, _ = m.Hwnd().MessageBox(o.t("gui.selectFiles"), o.t("gui.title"), co.MB_ICONINFORMATION)
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
	toOrig.On().BnClicked(func() { doRestore([]string{"--original"}) })
	toDir.On().BnClicked(func() {
		dir := pickFolder(uintptr(m.Hwnd()), o.t("gui.restoreTo"))
		if dir == "" {
			dir = desktopRestored()
		}
		doRestore([]string{"--to", dir})
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
	o.watchTimeout(m)
	open := button(m, o.t("gui.openLog"), 20, 15, 320)
	label(m, o.t("gui.language"), 20, 65, 100, 20)
	lang := combo(m, 120, 62, 120, "English", "Nederlands")
	if o.main.p.Lang == "nl" {
		lang.SelectIndex(1)
	}
	save := button(m, o.t("gui.save"), 250, 60, 90)
	open.On().BnClicked(func() {
		cfg := o.loadConfig()
		dir := cfg.LogPath
		if dir == "" {
			dir = config.Dir()
		}
		_ = shellExecute("open", filepath.Join(dir, config.LogFileName))
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
