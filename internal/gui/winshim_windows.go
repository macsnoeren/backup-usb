package gui

import (
	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/win"
)

type (
	winNMLISTVIEW     = win.NMLISTVIEW
	winNMITEMACTIVATE = win.NMITEMACTIVATE
)

// parentHwnd returns the window handle of a parent.
func parentHwnd(p ui.Parent) win.HWND { return p.Hwnd() }
