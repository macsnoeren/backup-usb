package gui

import (
	"github.com/rodrigocfd/windigo/co"
	"github.com/rodrigocfd/windigo/ui"
)

// newModal creates a modal window.
func newModal(parent ui.Parent, title string, w, h int) *ui.Modal {
	return ui.NewModal(parent, ui.OptsModal().Title(title).Size(ui.Dpi(w, h)))
}

func closeModal(m *ui.Modal) { _, _ = m.Hwnd().SendMessage(co.WM_CLOSE, 0, 0) }

func label(p ui.Parent, text string, x, y, w, h int) *ui.Static {
	return ui.NewStatic(p, ui.OptsStatic().Text(text).Position(ui.Dpi(x, y)).Size(ui.Dpi(w, h)))
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

func combo(p ui.Parent, x, y, w int, items ...string) *ui.ComboBox {
	return ui.NewComboBox(p, ui.OptsComboBox().Position(ui.Dpi(x, y)).Width(ui.DpiX(w)).CtrlStyle(co.CBS_DROPDOWNLIST).Texts(items...).Select(0))
}

// takeSecret reads a password field into a byte slice and clears the
// field. The control's own text and Go's string copy cannot be wiped; they
// live only as long as this window.
func takeSecret(e *ui.Edit) []byte {
	b := []byte(e.Text())
	e.SetText("")
	return b
}

// askText asks for one line of text.
func askText(parent ui.Parent, title, prompt, initial string, p func(string, ...any) string) (string, bool) {
	m := newModal(parent, title, 380, 130)
	label(m, prompt, 10, 10, 360, 20)
	e := edit(m, initial, 10, 35, 360, false)
	ok := button(m, p("gui.btn.ok"), 200, 75, 80)
	cancel := button(m, p("gui.btn.close"), 290, 75, 80)
	var result string
	var done bool
	ok.On().BnClicked(func() { result, done = e.Text(), true; closeModal(m) })
	cancel.On().BnClicked(func() { closeModal(m) })
	m.ShowModal()
	return result, done
}
